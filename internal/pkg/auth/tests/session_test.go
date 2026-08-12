package tests

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	. "github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/redis/go-redis/v9"
)

// The following mirror internal helpers from the auth package so this external
// test package can compute the same Redis keys for direct assertions.
func sessionKey(sessionID string) string {
	return "auth:session:" + sessionID
}

func userSessionsKey(userID int64) string {
	return fmt.Sprintf("auth:user:sessions:%d", userID)
}

func userRevokedBeforeKey(userID int64) string {
	return fmt.Sprintf("auth:user:revoked_before:%d", userID)
}

// testClient mirrors the redis client backing the SessionManager under test so
// helpers can seed/inspect keys directly without touching unexported fields.
var testClient *redis.Client

func newTestManager(t *testing.T) (*SessionManager, *miniredis.Miniredis, *redis.Client) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis start: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	testClient = client

	return NewSessionManager(client), mr, client
}

func assertCode(t *testing.T, err error, want errconstant.ErrorCode) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error with code %s, got nil", want)
	}

	var ae *apperror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("expected *apperror.Error, got %T (%v)", err, err)
	}

	if ae.Code != want {
		t.Fatalf("expected code %s, got %s", want, ae.Code)
	}
}

func seedSession(t *testing.T, mgr *SessionManager, id string, userID int64, fields map[string]any) {
	t.Helper()
	ctx := context.Background()
	if err := testClient.HSet(ctx, sessionKey(id), fields).Err(); err != nil {
		t.Fatalf("seed HSet: %v", err)
	}
}

func addToIndex(t *testing.T, mgr *SessionManager, userID int64, ids ...string) {
	t.Helper()
	ctx := context.Background()
	if err := testClient.SAdd(ctx, userSessionsKey(userID), ids).Err(); err != nil {
		t.Fatalf("seed SAdd: %v", err)
	}
}

// ---------------------------------------------------------------------------
// HashToken
// ---------------------------------------------------------------------------

func TestHashToken(t *testing.T) {
	t.Run("same input generates same hash", func(t *testing.T) {
		a := HashToken("refresh-token-1")
		b := HashToken("refresh-token-1")
		if a != b {
			t.Fatalf("expected stable hash, got %q and %q", a, b)
		}
	})

	t.Run("different tokens generate different hashes", func(t *testing.T) {
		a := HashToken("refresh-token-1")
		b := HashToken("refresh-token-2")
		if a == b {
			t.Fatalf("expected distinct hashes for distinct tokens")
		}
	})

	t.Run("output is not the plaintext token", func(t *testing.T) {
		token := "super-secret-plaintext"
		if HashToken(token) == token {
			t.Fatalf("hash must not equal plaintext")
		}
	})
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func newSession(userID int64, id string, now time.Time) Session {
	return Session{
		ID:          id,
		UserID:      userID,
		RefreshHash: HashToken("refresh-" + id),
		IPAddress:   "203.0.113.7",
		UserAgent:   "test-agent",
		CreatedAt:   now,
		ExpiresAt:   now.Add(7 * 24 * time.Hour).Truncate(time.Second),
	}
}

func TestCreate(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)

	t.Run("successfully creates session", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		ctx := context.Background()
		sess := newSession(1, "sid-A", now)

		if err := mgr.Create(ctx, sess); err != nil {
			t.Fatalf("Create: %v", err)
		}

		exists, err := client.Exists(ctx, sessionKey("sid-A")).Result()
		if err != nil {
			t.Fatalf("Exists: %v", err)
		}
		if exists != 1 {
			t.Fatalf("expected session key to exist, got %d", exists)
		}
	})

	t.Run("stores user_id", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		ctx := context.Background()
		if err := mgr.Create(ctx, newSession(42, "sid-B", now)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := client.HGet(ctx, sessionKey("sid-B"), "user_id").Result()
		if err != nil {
			t.Fatalf("HGet: %v", err)
		}
		if got != "42" {
			t.Fatalf("expected user_id 42, got %q", got)
		}
	})

	t.Run("stores refresh_hash", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		ctx := context.Background()
		if err := mgr.Create(ctx, newSession(1, "sid-C", now)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := client.HGet(ctx, sessionKey("sid-C"), "refresh_hash").Result()
		if err != nil {
			t.Fatalf("HGet: %v", err)
		}
		want := HashToken("refresh-sid-C")
		if got != want {
			t.Fatalf("expected refresh_hash %q, got %q", want, got)
		}
	})

	t.Run("stores ip address", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		ctx := context.Background()
		if err := mgr.Create(ctx, newSession(1, "sid-D", now)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := client.HGet(ctx, sessionKey("sid-D"), "ip_address").Result()
		if err != nil {
			t.Fatalf("HGet: %v", err)
		}
		if got != "203.0.113.7" {
			t.Fatalf("expected ip 203.0.113.7, got %q", got)
		}
	})

	t.Run("stores user agent", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		ctx := context.Background()
		if err := mgr.Create(ctx, newSession(1, "sid-E", now)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := client.HGet(ctx, sessionKey("sid-E"), "user_agent").Result()
		if err != nil {
			t.Fatalf("HGet: %v", err)
		}
		if got != "test-agent" {
			t.Fatalf("expected user agent test-agent, got %q", got)
		}
	})

	t.Run("stores created_at as unix milli", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		ctx := context.Background()
		if err := mgr.Create(ctx, newSession(1, "sid-F", now)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := client.HGet(ctx, sessionKey("sid-F"), "created_at").Int64()
		if err != nil {
			t.Fatalf("HGet: %v", err)
		}
		if got != now.UnixMilli() {
			t.Fatalf("expected created_at %d, got %d", now.UnixMilli(), got)
		}
	})

	t.Run("stores expires_at as unix seconds", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		ctx := context.Background()
		sess := newSession(1, "sid-G", now)
		if err := mgr.Create(ctx, sess); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := client.HGet(ctx, sessionKey("sid-G"), "expires_at").Int64()
		if err != nil {
			t.Fatalf("HGet: %v", err)
		}
		if got != sess.ExpiresAt.Unix() {
			t.Fatalf("expected expires_at %d, got %d", sess.ExpiresAt.Unix(), got)
		}
	})

	t.Run("adds session id to user-session index", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		ctx := context.Background()
		if err := mgr.Create(ctx, newSession(1, "sid-H", now)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		members, err := client.SMembers(ctx, userSessionsKey(1)).Result()
		if err != nil {
			t.Fatalf("SMembers: %v", err)
		}
		if len(members) != 1 || members[0] != "sid-H" {
			t.Fatalf("expected index to contain sid-H, got %v", members)
		}
	})

	t.Run("session key has ttl", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		ctx := context.Background()
		if err := mgr.Create(ctx, newSession(1, "sid-I", now)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		ttl, err := client.PTTL(ctx, sessionKey("sid-I")).Result()
		if err != nil {
			t.Fatalf("PTTL: %v", err)
		}
		if ttl <= 0 || ttl < 6*24*time.Hour {
			t.Fatalf("expected session ttl ~7d, got %v", ttl)
		}
	})

	t.Run("index ttl is not shortened by a shorter session", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		ctx := context.Background()

		long := newSession(1, "sid-long", now)
		if err := mgr.Create(ctx, long); err != nil {
			t.Fatalf("Create long: %v", err)
		}

		indexTTLBefore, err := client.PTTL(ctx, userSessionsKey(1)).Result()
		if err != nil {
			t.Fatalf("PTTL index: %v", err)
		}

		short := Session{
			ID:          "sid-short",
			UserID:      1,
			RefreshHash: HashToken("refresh-short"),
			CreatedAt:   now,
			ExpiresAt:   now.Add(1 * time.Hour).Truncate(time.Second),
		}
		if err := mgr.Create(ctx, short); err != nil {
			t.Fatalf("Create short: %v", err)
		}

		indexTTLAfter, err := client.PTTL(ctx, userSessionsKey(1)).Result()
		if err != nil {
			t.Fatalf("PTTL index: %v", err)
		}

		if indexTTLBefore < 6*24*time.Hour {
			t.Fatalf("index ttl before too small: %v", indexTTLBefore)
		}
		if indexTTLAfter < 6*24*time.Hour {
			t.Fatalf("index ttl was shortened by shorter session: %v", indexTTLAfter)
		}
	})

	t.Run("rejects an already-expired session", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		ctx := context.Background()
		sess := Session{
			ID:          "sid-expired",
			UserID:      1,
			RefreshHash: HashToken("refresh-expired"),
			CreatedAt:   now.Add(-2 * time.Hour),
			ExpiresAt:   now.Add(-1 * time.Hour),
		}
		err := mgr.Create(ctx, sess)
		if err == nil {
			t.Fatalf("expected error for expired session")
		}
		assertCode(t, err, errconstant.ErrCodeAuthInvalidToken)

		exists, err := client.Exists(ctx, sessionKey("sid-expired")).Result()
		if err != nil {
			t.Fatalf("Exists: %v", err)
		}
		if exists != 0 {
			t.Fatalf("expired session must not be persisted")
		}
	})

	t.Run("stores empty ip and unicode user agent", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		ctx := context.Background()
		sess := Session{
			ID:          "sid-unicode",
			UserID:      1,
			RefreshHash: HashToken("refresh-unicode"),
			IPAddress:   "",
			UserAgent:   "Mozilla/5.0 (日本語) 🦄",
			CreatedAt:   now,
			ExpiresAt:   now.Add(7 * 24 * time.Hour).Truncate(time.Second),
		}
		if err := mgr.Create(ctx, sess); err != nil {
			t.Fatalf("Create: %v", err)
		}
		ua, err := client.HGet(ctx, sessionKey("sid-unicode"), "user_agent").Result()
		if err != nil {
			t.Fatalf("HGet: %v", err)
		}
		if ua != sess.UserAgent {
			t.Fatalf("expected unicode ua %q, got %q", sess.UserAgent, ua)
		}
		ip, err := client.HGet(ctx, sessionKey("sid-unicode"), "ip_address").Result()
		if err != nil {
			t.Fatalf("HGet: %v", err)
		}
		if ip != "" {
			t.Fatalf("expected empty ip, got %q", ip)
		}
	})
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

func TestValidate(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	createdMs := now.UnixMilli()
	expiresUnix := now.Add(7 * 24 * time.Hour).Truncate(time.Second).Unix()

	t.Run("valid session succeeds", func(t *testing.T) {
		mgr, _, _ := newTestManager(t)
		seedSession(t, mgr, "sid-V", 1, map[string]any{
			"user_id":    1,
			"created_at": createdMs,
			"expires_at": expiresUnix,
		})
		if err := mgr.Validate(ctx, "sid-V", 1); err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
	})

	t.Run("missing session returns invalid token", func(t *testing.T) {
		mgr, _, _ := newTestManager(t)
		err := mgr.Validate(ctx, "sid-missing", 1)
		if err == nil {
			t.Fatalf("expected error for missing session")
		}
		assertCode(t, err, errconstant.ErrCodeAuthInvalidToken)
	})

	t.Run("wrong user id returns invalid token", func(t *testing.T) {
		mgr, _, _ := newTestManager(t)
		seedSession(t, mgr, "sid-W", 1, map[string]any{
			"user_id":    1,
			"created_at": createdMs,
			"expires_at": expiresUnix,
		})
		err := mgr.Validate(ctx, "sid-W", 2)
		if err == nil {
			t.Fatalf("expected error for wrong user")
		}
		assertCode(t, err, errconstant.ErrCodeAuthInvalidToken)
	})

	t.Run("invalid/malformed user id is rejected", func(t *testing.T) {
		mgr, _, _ := newTestManager(t)
		seedSession(t, mgr, "sid-MU", 1, map[string]any{
			"user_id":    "not-a-number",
			"created_at": createdMs,
			"expires_at": expiresUnix,
		})
		err := mgr.Validate(ctx, "sid-MU", 1)
		if err == nil {
			t.Fatalf("expected error for malformed user id")
		}
		assertCode(t, err, errconstant.ErrCodeAuthInvalidToken)
	})

	t.Run("invalid/malformed created_at is rejected", func(t *testing.T) {
		mgr, _, _ := newTestManager(t)
		seedSession(t, mgr, "sid-MC", 1, map[string]any{
			"user_id":    1,
			"created_at": "garbage",
			"expires_at": expiresUnix,
		})
		err := mgr.Validate(ctx, "sid-MC", 1)
		if err == nil {
			t.Fatalf("expected error for malformed created_at")
		}
		assertCode(t, err, errconstant.ErrCodeAuthInvalidToken)
	})

	t.Run("session created before revoked_before is rejected", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		revokedAt := now.UnixMilli()
		if err := client.Set(ctx, userRevokedBeforeKey(1), revokedAt, time.Hour).Err(); err != nil {
			t.Fatalf("Set: %v", err)
		}
		seedSession(t, mgr, "sid-BEFORE", 1, map[string]any{
			"user_id":    1,
			"created_at": revokedAt - 1000,
			"expires_at": expiresUnix,
		})
		err := mgr.Validate(ctx, "sid-BEFORE", 1)
		if err == nil {
			t.Fatalf("expected error for session before revoked_before")
		}
		assertCode(t, err, errconstant.ErrCodeAuthInvalidToken)
	})

	t.Run("session created after revoked_before is accepted", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		revokedAt := now.UnixMilli()
		if err := client.Set(ctx, userRevokedBeforeKey(1), revokedAt, time.Hour).Err(); err != nil {
			t.Fatalf("Set: %v", err)
		}
		seedSession(t, mgr, "sid-AFTER", 1, map[string]any{
			"user_id":    1,
			"created_at": revokedAt + 1000,
			"expires_at": expiresUnix,
		})
		if err := mgr.Validate(ctx, "sid-AFTER", 1); err != nil {
			t.Fatalf("expected accepted, got %v", err)
		}
	})

	t.Run("equal timestamps are accepted (new session race)", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		revokedAt := now.UnixMilli()
		if err := client.Set(ctx, userRevokedBeforeKey(1), revokedAt, time.Hour).Err(); err != nil {
			t.Fatalf("Set: %v", err)
		}
		seedSession(t, mgr, "sid-EQ", 1, map[string]any{
			"user_id":    1,
			"created_at": revokedAt,
			"expires_at": expiresUnix,
		})
		// createdAt == revokedBefore: must be accepted so a freshly logged-in
		// user is never locked out by the same-millisecond race.
		if err := mgr.Validate(ctx, "sid-EQ", 1); err != nil {
			t.Fatalf("expected accepted for equal timestamps, got %v", err)
		}
	})

	t.Run("missing user id is rejected", func(t *testing.T) {
		mgr, _, _ := newTestManager(t)
		seedSession(t, mgr, "sid-Z", 1, map[string]any{
			"user_id":    1,
			"created_at": createdMs,
			"expires_at": expiresUnix,
		})
		err := mgr.Validate(ctx, "sid-Z", 0)
		if err == nil {
			t.Fatalf("expected error for zero user id")
		}
		assertCode(t, err, errconstant.ErrCodeAuthInvalidToken)
	})
}

// ---------------------------------------------------------------------------
// Rotate
// ---------------------------------------------------------------------------

func seedRefreshSession(t *testing.T, mgr *SessionManager, id string, userID int64, refreshHash string) {
	t.Helper()
	seedSession(t, mgr, id, userID, map[string]any{
		"user_id":      userID,
		"refresh_hash": refreshHash,
	})
	addToIndex(t, mgr, userID, id)
}

func TestRotate(t *testing.T) {
	ctx := context.Background()

	t.Run("correct refresh hash rotates successfully", func(t *testing.T) {
		mgr, _, _ := newTestManager(t)
		seedRefreshSession(t, mgr, "sid-R1", 1, "HASH_A")
		if err := mgr.Rotate(ctx, "sid-R1", 1, "HASH_A", "HASH_B"); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})

	t.Run("refresh_hash actually changes", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		seedRefreshSession(t, mgr, "sid-R2", 1, "HASH_A")
		if err := mgr.Rotate(ctx, "sid-R2", 1, "HASH_A", "HASH_B"); err != nil {
			t.Fatalf("Rotate: %v", err)
		}
		got, err := client.HGet(ctx, sessionKey("sid-R2"), "refresh_hash").Result()
		if err != nil {
			t.Fatalf("HGet: %v", err)
		}
		if got != "HASH_B" {
			t.Fatalf("expected HASH_B, got %q", got)
		}
	})

	t.Run("rotated_at is stored", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		seedRefreshSession(t, mgr, "sid-R3", 1, "HASH_A")
		if err := mgr.Rotate(ctx, "sid-R3", 1, "HASH_A", "HASH_B"); err != nil {
			t.Fatalf("Rotate: %v", err)
		}
		got, err := client.HGet(ctx, sessionKey("sid-R3"), "rotated_at").Result()
		if err != nil {
			t.Fatalf("HGet: %v", err)
		}
		if got == "" {
			t.Fatalf("expected rotated_at to be set")
		}
	})

	t.Run("session id remains unchanged", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		seedRefreshSession(t, mgr, "sid-R4", 1, "HASH_A")
		if err := mgr.Rotate(ctx, "sid-R4", 1, "HASH_A", "HASH_B"); err != nil {
			t.Fatalf("Rotate: %v", err)
		}
		exists, err := client.Exists(ctx, sessionKey("sid-R4")).Result()
		if err != nil {
			t.Fatalf("Exists: %v", err)
		}
		if exists != 1 {
			t.Fatalf("expected session key to persist, got %d", exists)
		}
	})

	t.Run("missing session returns invalid refresh token", func(t *testing.T) {
		mgr, _, _ := newTestManager(t)
		err := mgr.Rotate(ctx, "sid-R5", 1, "HASH_A", "HASH_B")
		if err == nil {
			t.Fatalf("expected error for missing session")
		}
		assertCode(t, err, errconstant.ErrCodeAuthInvalidRefreshToken)
	})

	t.Run("wrong user cannot rotate another users session", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		seedRefreshSession(t, mgr, "sid-R6", 1, "HASH_A")
		// Caller claims userID=2 while the session belongs to user 1.
		err := mgr.Rotate(ctx, "sid-R6", 2, "HASH_A", "HASH_B")
		if err == nil {
			t.Fatalf("expected error for wrong user")
		}
		assertCode(t, err, errconstant.ErrCodeAuthInvalidRefreshToken)
		// Ownership mismatch must NOT delete or modify the session.
		got, gerr := client.HGet(ctx, sessionKey("sid-R6"), "refresh_hash").Result()
		if gerr != nil {
			t.Fatalf("HGet: %v", gerr)
		}
		if got != "HASH_A" {
			t.Fatalf("session must remain untouched, got %q", got)
		}
	})

	t.Run("old refresh token reuse is detected and revokes", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		seedRefreshSession(t, mgr, "sid-R7", 1, "HASH_A")
		// First rotation succeeds.
		if err := mgr.Rotate(ctx, "sid-R7", 1, "HASH_A", "HASH_B"); err != nil {
			t.Fatalf("first Rotate: %v", err)
		}
		// Replaying the old token HASH_A must now be rejected.
		err := mgr.Rotate(ctx, "sid-R7", 1, "HASH_A", "HASH_C")
		if err == nil {
			t.Fatalf("expected reuse to be rejected")
		}
		assertCode(t, err, errconstant.ErrCodeAuthInvalidRefreshToken)

		exists, eerr := client.Exists(ctx, sessionKey("sid-R7")).Result()
		if eerr != nil {
			t.Fatalf("Exists: %v", eerr)
		}
		if exists != 0 {
			t.Fatalf("reused session key must be deleted, got %d", exists)
		}
		ismem, merr := client.SIsMember(ctx, userSessionsKey(1), "sid-R7").Result()
		if merr != nil {
			t.Fatalf("SIsMember: %v", merr)
		}
		if ismem {
			t.Fatalf("reused session must be removed from index")
		}
	})

	t.Run("second rotation with same old hash cannot both succeed", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		seedRefreshSession(t, mgr, "sid-R8", 1, "HASH_A")

		if err := mgr.Rotate(ctx, "sid-R8", 1, "HASH_A", "HASH_B"); err != nil {
			t.Fatalf("first Rotate should succeed: %v", err)
		}
		// Concurrent-like replay of the already-used HASH_A.
		if err := mgr.Rotate(ctx, "sid-R8", 1, "HASH_A", "HASH_C"); err == nil {
			t.Fatalf("second Rotate with old hash must fail")
		}

		exists, eerr := client.Exists(ctx, sessionKey("sid-R8")).Result()
		if eerr != nil {
			t.Fatalf("Exists: %v", eerr)
		}
		if exists != 0 {
			t.Fatalf("expected session deleted after reuse, got %d", exists)
		}
		if mem, _ := client.SIsMember(ctx, userSessionsKey(1), "sid-R8").Result(); mem {
			t.Fatalf("expected session removed from index")
		}
	})
}

func TestRotate_ConcurrentReuseDetection(t *testing.T) {
	mgr, _, _ := newTestManager(t)
	ctx := context.Background()
	seedRefreshSession(t, mgr, "sid-CONC", 1, "HASH_A")

	const n = 10
	results := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			newHash := "HASH_NEW_" + strconv.Itoa(i)
			results <- mgr.Rotate(ctx, "sid-CONC", 1, "HASH_A", newHash)
		}(i)
	}

	success := 0
	for i := 0; i < n; i++ {
		if err := <-results; err == nil {
			success++
		}
	}
	// Exactly one rotation wins; every replay of HASH_A is a reuse detection.
	if success != 1 {
		t.Fatalf("expected exactly one successful rotation, got %d", success)
	}
}

// ---------------------------------------------------------------------------
// Revoke
// ---------------------------------------------------------------------------

func TestRevoke(t *testing.T) {
	ctx := context.Background()

	t.Run("revokes one session removes key and index member", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		seedRefreshSession(t, mgr, "sid-RA", 1, "HASH_A")
		if err := mgr.Revoke(ctx, "sid-RA", 1); err != nil {
			t.Fatalf("Revoke: %v", err)
		}
		exists, err := client.Exists(ctx, sessionKey("sid-RA")).Result()
		if err != nil {
			t.Fatalf("Exists: %v", err)
		}
		if exists != 0 {
			t.Fatalf("expected session key deleted, got %d", exists)
		}
		ismem, err := client.SIsMember(ctx, userSessionsKey(1), "sid-RA").Result()
		if err != nil {
			t.Fatalf("SIsMember: %v", err)
		}
		if ismem {
			t.Fatalf("expected session removed from index")
		}
	})

	t.Run("does not remove another session of same user", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		seedRefreshSession(t, mgr, "session-A", 1, "HASH_A")
		seedRefreshSession(t, mgr, "session-B", 1, "HASH_B")

		if err := mgr.Revoke(ctx, "session-A", 1); err != nil {
			t.Fatalf("Revoke: %v", err)
		}

		existsB, err := client.Exists(ctx, sessionKey("session-B")).Result()
		if err != nil {
			t.Fatalf("Exists B: %v", err)
		}
		if existsB != 1 {
			t.Fatalf("session-B must remain, got %d", existsB)
		}
		ismemB, err := client.SIsMember(ctx, userSessionsKey(1), "session-B").Result()
		if err != nil {
			t.Fatalf("SIsMember B: %v", err)
		}
		if !ismemB {
			t.Fatalf("session-B must remain in index")
		}
		ismemA, _ := client.SIsMember(ctx, userSessionsKey(1), "session-A").Result()
		if ismemA {
			t.Fatalf("session-A must be removed from index")
		}
	})

	t.Run("missing session is idempotent", func(t *testing.T) {
		mgr, _, _ := newTestManager(t)
		if err := mgr.Revoke(ctx, "sid-gone", 1); err != nil {
			t.Fatalf("expected nil for missing session, got %v", err)
		}
	})

	t.Run("user cannot revoke another users session", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		seedRefreshSession(t, mgr, "sid-OTHER", 1, "HASH_A")
		err := mgr.Revoke(ctx, "sid-OTHER", 2)
		if err == nil {
			t.Fatalf("expected error for wrong user")
		}
		assertCode(t, err, errconstant.ErrCodeAuthInvalidToken)
		// Session must remain intact.
		exists, err := client.Exists(ctx, sessionKey("sid-OTHER")).Result()
		if err != nil {
			t.Fatalf("Exists: %v", err)
		}
		if exists != 1 {
			t.Fatalf("session must not be deleted by wrong user, got %d", exists)
		}
	})
}

// ---------------------------------------------------------------------------
// RevokeAll
// ---------------------------------------------------------------------------

func TestRevokeAll(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	t.Run("revokes all sessions of a user and marks revoked_before", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		seedRefreshSession(t, mgr, "sid-U1A", 1, "HASH_A")
		seedRefreshSession(t, mgr, "sid-U1B", 1, "HASH_B")
		seedRefreshSession(t, mgr, "sid-U2", 2, "HASH_X")

		if err := mgr.RevokeAll(ctx, 1, 7*24*time.Hour); err != nil {
			t.Fatalf("RevokeAll: %v", err)
		}

		for _, id := range []string{"sid-U1A", "sid-U1B"} {
			exists, err := client.Exists(ctx, sessionKey(id)).Result()
			if err != nil {
				t.Fatalf("Exists %s: %v", id, err)
			}
			if exists != 0 {
				t.Fatalf("expected %s deleted, got %d", id, exists)
			}
		}

		indexExists, err := client.Exists(ctx, userSessionsKey(1)).Result()
		if err != nil {
			t.Fatalf("Exists index: %v", err)
		}
		if indexExists != 0 {
			t.Fatalf("expected user-1 index deleted")
		}

		rb, err := client.Exists(ctx, userRevokedBeforeKey(1)).Result()
		if err != nil {
			t.Fatalf("Exists revoked_before: %v", err)
		}
		if rb != 1 {
			t.Fatalf("expected revoked_before marker to exist")
		}

		// Other user untouched.
		existsOther, err := client.Exists(ctx, sessionKey("sid-U2")).Result()
		if err != nil {
			t.Fatalf("Exists U2: %v", err)
		}
		if existsOther != 1 {
			t.Fatalf("user-2 session must remain, got %d", existsOther)
		}
	})

	t.Run("subsequent validate on old session fails", func(t *testing.T) {
		mgr, _, _ := newTestManager(t)
		seedRefreshSession(t, mgr, "sid-OLD", 1, "HASH_A")
		if err := mgr.RevokeAll(ctx, 1, 7*24*time.Hour); err != nil {
			t.Fatalf("RevokeAll: %v", err)
		}
		err := mgr.Validate(ctx, "sid-OLD", 1)
		if err == nil {
			t.Fatalf("expected validation to fail after logout-all")
		}
		assertCode(t, err, errconstant.ErrCodeAuthInvalidToken)
	})

	t.Run("new session after logout-all validates successfully", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		if err := mgr.RevokeAll(ctx, 1, 7*24*time.Hour); err != nil {
			t.Fatalf("RevokeAll: %v", err)
		}
		// Capture the logout-all marker so we can prove the new session is newer.
		rbRaw, err := client.Get(ctx, userRevokedBeforeKey(1)).Result()
		if err != nil {
			t.Fatalf("Get revoked_before: %v", err)
		}
		revokedAt, err := strconv.ParseInt(rbRaw, 10, 64)
		if err != nil {
			t.Fatalf("parse revoked_before: %v", err)
		}

		newSess := Session{
			ID:          "sid-NEW",
			UserID:      1,
			RefreshHash: HashToken("refresh-new"),
			CreatedAt:   time.UnixMilli(revokedAt + 1000),
			ExpiresAt:   now.Add(7 * 24 * time.Hour).Truncate(time.Second),
		}
		if err := mgr.Create(ctx, newSess); err != nil {
			t.Fatalf("Create new: %v", err)
		}
		if err := mgr.Validate(ctx, "sid-NEW", 1); err != nil {
			t.Fatalf("expected new session to validate, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func sortedIDs(sessions []SessionInfo) []string {
	ids := make([]string, 0, len(sessions))
	for _, s := range sessions {
		ids = append(ids, s.ID)
	}
	return ids
}

func TestList(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	expires := now.Add(7 * 24 * time.Hour).Truncate(time.Second)

	t.Run("empty index returns empty slice", func(t *testing.T) {
		mgr, _, _ := newTestManager(t)
		sessions, err := mgr.List(ctx, 1)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(sessions) != 0 {
			t.Fatalf("expected empty slice, got %v", sessions)
		}
	})

	t.Run("lists active sessions with metadata", func(t *testing.T) {
		mgr, _, _ := newTestManager(t)
		if err := mgr.Create(ctx, Session{
			ID: "sid-L1", UserID: 1, RefreshHash: "rh",
			IPAddress: "198.51.100.9", UserAgent: "agent-one",
			CreatedAt: now, ExpiresAt: expires,
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := mgr.Create(ctx, Session{
			ID: "sid-L2", UserID: 1, RefreshHash: "rh",
			IPAddress: "198.51.100.10", UserAgent: "agent-two",
			CreatedAt: now, ExpiresAt: expires,
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}

		sessions, err := mgr.List(ctx, 1)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(sessions) != 2 {
			t.Fatalf("expected 2 sessions, got %d", len(sessions))
		}

		byID := make(map[string]SessionInfo)
		for _, s := range sessions {
			byID[s.ID] = s
		}
		a, ok := byID["sid-L1"]
		if !ok {
			t.Fatalf("sid-L1 missing")
		}
		if a.IPAddress != "198.51.100.9" || a.UserAgent != "agent-one" {
			t.Fatalf("metadata mismatch: %+v", a)
		}
		if !a.CreatedAt.Equal(now) {
			t.Fatalf("createdAt mismatch: %v vs %v", a.CreatedAt, now)
		}
		if !a.ExpiresAt.Equal(expires) {
			t.Fatalf("expiresAt mismatch: %v vs %v", a.ExpiresAt, expires)
		}
	})

	t.Run("missing redis session is excluded and index cleaned", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		addToIndex(t, mgr, 1, "sid-PHANTOM")
		sessions, err := mgr.List(ctx, 1)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(sessions) != 0 {
			t.Fatalf("expected no sessions, got %v", sessions)
		}
		ismem, err := client.SIsMember(ctx, userSessionsKey(1), "sid-PHANTOM").Result()
		if err != nil {
			t.Fatalf("SIsMember: %v", err)
		}
		if ismem {
			t.Fatalf("stale session id must be removed from index")
		}
	})

	t.Run("mismatched user id is excluded and cleaned", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		seedSession(t, mgr, "sid-FOREIGN", 99, map[string]any{
			"user_id":    99,
			"created_at": now.UnixMilli(),
			"expires_at": expires.Unix(),
		})
		addToIndex(t, mgr, 1, "sid-FOREIGN")
		sessions, err := mgr.List(ctx, 1)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, s := range sessions {
			if s.ID == "sid-FOREIGN" {
				t.Fatalf("foreign session must be excluded")
			}
		}
		ismem, err := client.SIsMember(ctx, userSessionsKey(1), "sid-FOREIGN").Result()
		if err != nil {
			t.Fatalf("SIsMember: %v", err)
		}
		if ismem {
			t.Fatalf("foreign session id must be removed from index")
		}
	})

	t.Run("malformed session data does not panic", func(t *testing.T) {
		mgr, _, client := newTestManager(t)
		seedSession(t, mgr, "sid-BAD", 1, map[string]any{
			"user_id":    1,
			"created_at": "nope",
			"expires_at": "nope",
		})
		addToIndex(t, mgr, 1, "sid-BAD")
		sessions, err := mgr.List(ctx, 1)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, s := range sessions {
			if s.ID == "sid-BAD" {
				t.Fatalf("malformed session must be excluded")
			}
		}
		ismem, err := client.SIsMember(ctx, userSessionsKey(1), "sid-BAD").Result()
		if err != nil {
			t.Fatalf("SIsMember: %v", err)
		}
		if ismem {
			t.Fatalf("malformed session id must be removed from index")
		}
	})

	t.Run("multiple sessions returned without assuming order", func(t *testing.T) {
		mgr, _, _ := newTestManager(t)
		for _, id := range []string{"sid-x", "sid-y", "sid-z"} {
			if err := mgr.Create(ctx, Session{
				ID: id, UserID: 1, RefreshHash: "rh",
				IPAddress: "10.0.0.1", UserAgent: "ua",
				CreatedAt: now, ExpiresAt: expires,
			}); err != nil {
				t.Fatalf("Create %s: %v", id, err)
			}
		}
		sessions, err := mgr.List(ctx, 1)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got := make(map[string]bool)
		for _, s := range sessions {
			got[s.ID] = true
		}
		for _, id := range []string{"sid-x", "sid-y", "sid-z"} {
			if !got[id] {
				t.Fatalf("expected %s in list, got %v", id, sortedIDs(sessions))
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Infrastructure failure classification
// ---------------------------------------------------------------------------

func TestServiceUnavailableOnRedisFailure(t *testing.T) {
	ctx := context.Background()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mgr := NewSessionManager(client)

	// Kill Redis so every call fails with a connection error.
	mr.Close()

	if err := mgr.Validate(ctx, "sid-any", 1); err == nil {
		t.Fatalf("expected error when Redis is down")
	} else {
		assertCode(t, err, errconstant.ErrCodeServiceUnavailable)
	}
}
