//go:build integration
// +build integration

package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/dto"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
)

// SCENARIO 1 — LOGIN SUCCESS
func TestAuth_LoginSuccess(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "login-success@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "")

	if lr.Auth.AccessToken == "" {
		t.Fatal("access_token empty")
	}
	if lr.Auth.RefreshToken == "" {
		t.Fatal("refresh_token empty")
	}
	if lr.Auth.AccessToken == lr.Auth.RefreshToken {
		t.Fatal("access and refresh tokens are identical")
	}
	if lr.Auth.TokenType != "Bearer" {
		t.Fatalf("token_type = %q, want Bearer", lr.Auth.TokenType)
	}
	if lr.Auth.ExpiresIn != int64(env.cfg.Auth.JWT.AccessExpiry.Seconds()) {
		t.Fatalf("expires_in = %d, want %d", lr.Auth.ExpiresIn, int64(env.cfg.Auth.JWT.AccessExpiry.Seconds()))
	}

	ctx := context.Background()
	key := sessionKey(lr.SID)

	if !env.exists(ctx, key) {
		t.Fatalf("session key %s not created", key)
	}

	fields := env.hGetAll(ctx, key)
	for _, f := range []string{"user_id", "refresh_hash", "ip_address", "user_agent", "created_at", "expires_at"} {
		if _, ok := fields[f]; !ok {
			t.Fatalf("session missing field %q: %v", f, fields)
		}
	}

	if fields["user_id"] != fmt.Sprintf("%d", lr.UserID) {
		t.Fatalf("user_id field = %q, want %d", fields["user_id"], lr.UserID)
	}

	// Raw refresh token must NEVER be stored; only its SHA-256 hash is.
	if fields["refresh_hash"] == lr.Auth.RefreshToken {
		t.Fatal("raw refresh token stored in Redis (must be hashed)")
	}

	for _, v := range fields {
		if v == lr.Auth.RefreshToken {
			t.Fatal("raw refresh token leaked into session hash")
		}
	}

	if !env.sIsMember(ctx, userSessionsKey(lr.UserID), lr.SID) {
		t.Fatal("SID not added to user-session index")
	}
}

// SCENARIO 2 — LOGIN INVALID PASSWORD
func TestAuth_LoginInvalidPassword(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "login-wrongpw@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	resp := env.loginRaw(t, email, "WrongPassword!", "")
	requireStatus(t, resp, http.StatusUnauthorized)
	requireCode(t, resp, "E_AUTH_1")

	ctx := context.Background()
	// No authenticated session should have been created. Confirm there is no
	// user-session index key (created only on a successful login).
	users, _ := env.redis.Keys(ctx, "auth:user:sessions:*").Result()
	if len(users) != 0 {
		t.Fatalf("unexpected session index keys after failed login: %v", users)
	}
}

// SCENARIO 3 — LOGIN UNKNOWN USER
func TestAuth_LoginUnknownUser(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	resp := env.loginRaw(t, "nobody@example.com", "Password123!", "")
	requireStatus(t, resp, http.StatusUnauthorized)
	// Same generic credential error as a wrong password (no user enumeration).
	requireCode(t, resp, "E_AUTH_1")
}

// SCENARIO 4 — ACCESS /auth/me
func TestAuth_MeSuccess(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "me-success@example.com"
	uid := env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "")

	resp := env.authed(http.MethodGet, "/auth/me", lr.Auth.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK)

	var user dto.User
	decode(t, resp, &user)

	if user.ID != uid {
		t.Fatalf("returned user id %d, want %d", user.ID, uid)
	}
	if user.Email != email {
		t.Fatalf("returned email %q", user.Email)
	}
}

// SCENARIO 5 — /auth/me WITHOUT TOKEN
func TestAuth_MeNoToken(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	resp := env.do("GET", "/auth/me", nil, nil)
	requireStatus(t, resp, http.StatusUnauthorized)
}

// SCENARIO 6 — /auth/me WITH INVALID TOKEN
func TestAuth_MeInvalidToken(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	resp := env.authed(http.MethodGet, "/auth/me", "invalid-token", nil)
	requireStatus(t, resp, http.StatusUnauthorized)
}

// SCENARIO 7 — REFRESH SUCCESS
func TestAuth_RefreshSuccess(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "refresh-success@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "")

	authB, resp := env.refresh(t, lr.Auth.RefreshToken)
	requireStatus(t, resp, http.StatusOK)

	if authB.AccessToken == "" || authB.RefreshToken == "" {
		t.Fatal("refresh returned empty tokens")
	}
	if authB.AccessToken == lr.Auth.AccessToken {
		t.Fatal("access token unchanged after refresh")
	}
	if authB.RefreshToken == lr.Auth.RefreshToken {
		t.Fatal("refresh token unchanged after refresh")
	}

	claimsA, _ := env.jwt.ParseAccessToken(lr.Auth.AccessToken)
	claimsB, _ := env.jwt.ParseAccessToken(authB.AccessToken)
	if claimsB.UserID != claimsA.UserID {
		t.Fatalf("user id changed: %d vs %d", claimsB.UserID, claimsA.UserID)
	}
	if claimsB.SessionID != claimsA.SessionID {
		t.Fatalf("session id changed: %q vs %q", claimsB.SessionID, claimsA.SessionID)
	}
	if claimsB.ID == claimsA.ID {
		t.Fatal("JTI (token id) did not change after refresh")
	}
	if claimsB.Type != "access" {
		t.Fatalf("access token type = %q", claimsB.Type)
	}

	refreshClaimsB, err := env.jwt.ParseRefreshToken(authB.RefreshToken)
	if err != nil {
		t.Fatalf("parse new refresh token: %v", err)
	}
	if refreshClaimsB.Type != "refresh" {
		t.Fatalf("refresh token type = %q", refreshClaimsB.Type)
	}
}

// SCENARIO 8 — ABSOLUTE REFRESH EXPIRY
func TestAuth_RefreshAbsoluteExpiry(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "refresh-absolute@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "")

	beforeClaims, _ := env.jwt.ParseRefreshToken(lr.Auth.RefreshToken)
	expiryA := beforeClaims.ExpiresAt.Time

	authB, resp := env.refresh(t, lr.Auth.RefreshToken)
	requireStatus(t, resp, http.StatusOK)

	afterClaims, _ := env.jwt.ParseRefreshToken(authB.RefreshToken)
	expiryB := afterClaims.ExpiresAt.Time

	if diff := expiryB.Sub(expiryA); diff < -2*time.Second || diff > 2*time.Second {
		t.Fatalf("refresh expiry not absolute: A=%v B=%v (diff %v)", expiryA, expiryB, diff)
	}

	// It must NOT have been extended to (refresh time + full TTL).
	// Absolute expiry means expiryB stays pinned to the login time; only a
	// sliding policy would push it past now + full TTL.
	if expiryB.After(time.Now().Add(env.cfg.Auth.JWT.RefreshExpiry)) {
		t.Fatalf("refresh expiry was extended (sliding) instead of absolute: %v", expiryB)
	}
}

// SCENARIO 9 — ACCESS AFTER REFRESH
func TestAuth_AccessAfterRefresh(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "access-after-refresh@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "")
	authB, resp := env.refresh(t, lr.Auth.RefreshToken)
	requireStatus(t, resp, http.StatusOK)

	me := env.authed(http.MethodGet, "/auth/me", authB.AccessToken, nil)
	requireStatus(t, me, http.StatusOK)
}

// SCENARIO 10 — OLD REFRESH TOKEN REUSE
func TestAuth_RefreshReuseRevokesSession(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "reuse@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "")
	sid := lr.SID

	// A -> B
	if _, resp := env.refresh(t, lr.Auth.RefreshToken); resp.StatusCode != http.StatusOK {
		t.Fatal("first refresh should succeed")
	}

	// Replay A: must fail and revoke the session.
	reuse := env.do(http.MethodPost, "/auth/refresh", payloads.RefreshTokenRequest{RefreshToken: lr.Auth.RefreshToken}, nil)
	requireStatus(t, reuse, http.StatusUnauthorized)
	requireCode(t, reuse, "E_AUTH_8")

	ctx := context.Background()
	if env.exists(ctx, sessionKey(sid)) {
		t.Fatal("reused session key must be deleted")
	}
	if env.sIsMember(ctx, userSessionsKey(lr.UserID), sid) {
		t.Fatal("reused SID must be removed from user-session index")
	}

	// Access token tied to the revoked session must now be rejected.
	me := env.authed(http.MethodGet, "/auth/me", lr.Auth.AccessToken, nil)
	requireStatus(t, me, http.StatusUnauthorized)
}

// SCENARIO 11 — DOUBLE REFRESH / CONCURRENT-LIKE ROTATION
func TestAuth_RefreshConcurrentOnlyOneSuccess(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "concurrent@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "")

	const n = 10
	results := make(chan int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := env.do(http.MethodPost, "/auth/refresh", payloads.RefreshTokenRequest{RefreshToken: lr.Auth.RefreshToken}, nil)
			results <- resp.StatusCode
		}()
	}
	wg.Wait()
	close(results)

	success := 0
	failure := 0
	for code := range results {
		if code == http.StatusOK {
			success++
		} else {
			failure++
		}
	}

	if success != 1 {
		t.Fatalf("expected exactly one successful rotation, got %d (failures %d)", success, failure)
	}
	if failure != n-1 {
		t.Fatalf("expected %d failures, got %d", n-1, failure)
	}
}

// SCENARIO 12 — LIST ACTIVE SESSIONS
func TestAuth_ListSessions(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "list@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "E2E-Device-List")

	resp := env.authed(http.MethodGet, "/auth/sessions", lr.Auth.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK)

	var sessions []dto.Session
	decode(t, resp, &sessions)

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if s.ID != lr.SID {
		t.Fatalf("session id = %q, want %q", s.ID, lr.SID)
	}
	if !s.Current {
		t.Fatal("current session not marked current")
	}
	if s.UserAgent != "E2E-Device-List" {
		t.Fatalf("user agent = %q", s.UserAgent)
	}
	if s.IPAddress == "" {
		t.Fatal("ip_address empty")
	}
	if s.CreatedAt.IsZero() {
		t.Fatal("created_at empty")
	}
	if s.ExpiresAt.IsZero() {
		t.Fatal("expires_at empty")
	}
}

// SCENARIO 13 — MULTIPLE DEVICES
func TestAuth_MultipleDevices(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "multi@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	devA := env.login(t, email, "Password123!", "E2E-Test-Device-A")
	devB := env.login(t, email, "Password123!", "E2E-Test-Device-B")

	// Inspect from A's perspective.
	respA := env.authed(http.MethodGet, "/auth/sessions", devA.Auth.AccessToken, nil)
	requireStatus(t, respA, http.StatusOK)
	var sessions []dto.Session
	decode(t, respA, &sessions)

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	currentCount := 0
	seenUA := map[string]bool{}
	seenID := map[string]bool{}
	for _, s := range sessions {
		if s.Current {
			currentCount++
			if s.ID != devA.SID {
				t.Fatalf("current session id = %q, want %q", s.ID, devA.SID)
			}
		}
		seenUA[s.UserAgent] = true
		seenID[s.ID] = true
	}
	if currentCount != 1 {
		t.Fatalf("expected exactly one current session, got %d", currentCount)
	}
	if !seenUA["E2E-Test-Device-A"] || !seenUA["E2E-Test-Device-B"] {
		t.Fatalf("both device user agents should appear: %v", seenUA)
	}
	if !seenID[devA.SID] || !seenID[devB.SID] {
		t.Fatalf("both device session ids should appear: %v", seenID)
	}
}

// SCENARIO 14 — REVOKE SPECIFIC DEVICE
func TestAuth_RevokeSpecificDevice(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "revoke-specific@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	devA := env.login(t, email, "Password123!", "E2E-Test-Device-A")
	devB := env.login(t, email, "Password123!", "E2E-Test-Device-B")

	// B revokes A.
	del := env.authed(http.MethodDelete, "/auth/sessions/"+devA.SID, devB.Auth.AccessToken, nil)
	requireStatus(t, del, http.StatusNoContent)

	meA := env.authed(http.MethodGet, "/auth/me", devA.Auth.AccessToken, nil)
	requireStatus(t, meA, http.StatusUnauthorized)

	meB := env.authed(http.MethodGet, "/auth/me", devB.Auth.AccessToken, nil)
	requireStatus(t, meB, http.StatusOK)

	ctx := context.Background()
	if env.sIsMember(ctx, userSessionsKey(devA.UserID), devA.SID) {
		t.Fatal("revoked device A still in index")
	}
	if !env.sIsMember(ctx, userSessionsKey(devB.UserID), devB.SID) {
		t.Fatal("device B should remain in index")
	}
}

// SCENARIO 15 — CANNOT REVOKE OTHER USER SESSION
func TestAuth_CannotRevokeOtherUserSession(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	emailA := "user-a@example.com"
	emailB := "user-b@example.com"
	env.createUserFixture(t, emailA, "Password123!", enums.StatusActive)
	env.createUserFixture(t, emailB, "Password123!", enums.StatusActive)

	userA := env.login(t, emailA, "Password123!", "E2E-A")
	userB := env.login(t, emailB, "Password123!", "E2E-B")

	// A tries to revoke B's session.
	del := env.authed(http.MethodDelete, "/auth/sessions/"+userB.SID, userA.Auth.AccessToken, nil)
	if del.StatusCode == http.StatusNoContent {
		t.Fatal("user A must not be able to revoke user B's session")
	}

	// B's session must remain valid.
	meB := env.authed(http.MethodGet, "/auth/me", userB.Auth.AccessToken, nil)
	requireStatus(t, meB, http.StatusOK)

	ctx := context.Background()
	if !env.sIsMember(ctx, userSessionsKey(userB.UserID), userB.SID) {
		t.Fatal("user B session should still be in index")
	}
}

// SCENARIO 16 — LOGOUT CURRENT SESSION
func TestAuth_LogoutCurrent(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "logout-current@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "")

	logout := env.authed(http.MethodPost, "/auth/logout", lr.Auth.AccessToken, nil)
	requireStatus(t, logout, http.StatusNoContent)

	me := env.authed(http.MethodGet, "/auth/me", lr.Auth.AccessToken, nil)
	requireStatus(t, me, http.StatusUnauthorized)

	ctx := context.Background()
	if env.exists(ctx, sessionKey(lr.SID)) {
		t.Fatal("session key not removed after logout")
	}
	if env.sIsMember(ctx, userSessionsKey(lr.UserID), lr.SID) {
		t.Fatal("SID not removed from index after logout")
	}
}

// SCENARIO 17 — LOGOUT IS SESSION-SPECIFIC
func TestAuth_LogoutIsSessionSpecific(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "logout-specific@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	devA := env.login(t, email, "Password123!", "E2E-A")
	devB := env.login(t, email, "Password123!", "E2E-B")

	logout := env.authed(http.MethodPost, "/auth/logout", devA.Auth.AccessToken, nil)
	requireStatus(t, logout, http.StatusNoContent)

	meA := env.authed(http.MethodGet, "/auth/me", devA.Auth.AccessToken, nil)
	requireStatus(t, meA, http.StatusUnauthorized)

	meB := env.authed(http.MethodGet, "/auth/me", devB.Auth.AccessToken, nil)
	requireStatus(t, meB, http.StatusOK)
}

// SCENARIO 18 — LOGOUT ALL
func TestAuth_LogoutAll(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "logout-all@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	devA := env.login(t, email, "Password123!", "E2E-A")
	devB := env.login(t, email, "Password123!", "E2E-B")

	logoutAll := env.authed(http.MethodPost, "/auth/logout-all", devA.Auth.AccessToken, nil)
	requireStatus(t, logoutAll, http.StatusNoContent)

	meA := env.authed(http.MethodGet, "/auth/me", devA.Auth.AccessToken, nil)
	requireStatus(t, meA, http.StatusUnauthorized)
	meB := env.authed(http.MethodGet, "/auth/me", devB.Auth.AccessToken, nil)
	requireStatus(t, meB, http.StatusUnauthorized)

	ctx := context.Background()
	if env.exists(ctx, sessionKey(devA.SID)) {
		t.Fatal("device A session key not removed")
	}
	if env.exists(ctx, sessionKey(devB.SID)) {
		t.Fatal("device B session key not removed")
	}
	if n, _ := env.redis.SCard(ctx, userSessionsKey(devA.UserID)).Result(); n != 0 {
		t.Fatalf("user session index not cleaned, members=%d", n)
	}
	// revoked_before marker should exist.
	if !env.exists(ctx, fmt.Sprintf("auth:user:revoked_before:%d", devA.UserID)) {
		t.Fatal("revoked_before marker missing after logout-all")
	}
}

// SCENARIO 19 — LOGIN AFTER LOGOUT ALL
func TestAuth_LoginAfterLogoutAll(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "login-after-logout-all@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	devA := env.login(t, email, "Password123!", "E2E-A")
	devB := env.login(t, email, "Password123!", "E2E-B")

	logoutAll := env.authed(http.MethodPost, "/auth/logout-all", devA.Auth.AccessToken, nil)
	requireStatus(t, logoutAll, http.StatusNoContent)

	// Both pre-existing sessions must now be invalid.
	for _, old := range []loginResult{devA, devB} {
		oldMe := env.authed(http.MethodGet, "/auth/me", old.Auth.AccessToken, nil)
		requireStatus(t, oldMe, http.StatusUnauthorized)
	}

	// Brand new login afterwards must work.
	devC := env.login(t, email, "Password123!", "E2E-C")
	me := env.authed(http.MethodGet, "/auth/me", devC.Auth.AccessToken, nil)
	requireStatus(t, me, http.StatusOK)
}

// SCENARIO 20 — REFRESH AFTER LOGOUT
func TestAuth_RefreshAfterLogout(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "refresh-after-logout@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "")

	logout := env.authed(http.MethodPost, "/auth/logout", lr.Auth.AccessToken, nil)
	requireStatus(t, logout, http.StatusNoContent)

	reuse := env.do(http.MethodPost, "/auth/refresh", payloads.RefreshTokenRequest{RefreshToken: lr.Auth.RefreshToken}, nil)
	requireStatus(t, reuse, http.StatusUnauthorized)
}

// SCENARIO 21 — REFRESH AFTER LOGOUT ALL
func TestAuth_RefreshAfterLogoutAll(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "refresh-after-logout-all@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	devA := env.login(t, email, "Password123!", "E2E-A")
	devB := env.login(t, email, "Password123!", "E2E-B")

	logoutAll := env.authed(http.MethodPost, "/auth/logout-all", devA.Auth.AccessToken, nil)
	requireStatus(t, logoutAll, http.StatusNoContent)

	for _, lr := range []loginResult{devA, devB} {
		reuse := env.do(http.MethodPost, "/auth/refresh", payloads.RefreshTokenRequest{RefreshToken: lr.Auth.RefreshToken}, nil)
		requireStatus(t, reuse, http.StatusUnauthorized)
	}
}

// SCENARIO 22 — REFRESH AFTER SPECIFIC DEVICE REVOCATION
func TestAuth_RefreshAfterSpecificRevoke(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "refresh-after-revoke@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	devA := env.login(t, email, "Password123!", "E2E-A")
	devB := env.login(t, email, "Password123!", "E2E-B")

	// B revokes A.
	del := env.authed(http.MethodDelete, "/auth/sessions/"+devA.SID, devB.Auth.AccessToken, nil)
	requireStatus(t, del, http.StatusNoContent)

	reuseA := env.do(http.MethodPost, "/auth/refresh", payloads.RefreshTokenRequest{RefreshToken: devA.Auth.RefreshToken}, nil)
	requireStatus(t, reuseA, http.StatusUnauthorized)

	authB, resp := env.refresh(t, devB.Auth.RefreshToken)
	requireStatus(t, resp, http.StatusOK)
	if authB.AccessToken == "" {
		t.Fatal("device B refresh should still work")
	}
}

// SCENARIO 23 — TOKEN TYPE CONFUSION
func TestAuth_TokenTypeConfusion(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "type-confusion@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "")

	// Refresh endpoint fed an ACCESS token -> rejected. Access tokens are
	// signed with a different secret, so this is an invalid refresh token.
	badRefresh := env.do(http.MethodPost, "/auth/refresh", payloads.RefreshTokenRequest{RefreshToken: lr.Auth.AccessToken}, nil)
	requireStatus(t, badRefresh, http.StatusUnauthorized)
	requireCode(t, badRefresh, "E_AUTH_8")

	// /auth/me fed a REFRESH token as Bearer -> rejected: signature check
	// against the access secret fails before any type inspection.
	badMe := env.authed(http.MethodGet, "/auth/me", lr.Auth.RefreshToken, nil)
	requireStatus(t, badMe, http.StatusUnauthorized)
	requireCode(t, badMe, "E_AUTH_5")
}

// SCENARIO 24 — TOKEN SIGNATURE / TAMPERING
func TestAuth_TokenTampering(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "tamper@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "")

	// Tamper with the access token signature.
	tampered := lr.Auth.AccessToken
	if len(tampered) > 5 {
		tampered = tampered[:len(tampered)-3] + "xyz"
	}
	me := env.authed(http.MethodGet, "/auth/me", tampered, nil)
	requireStatus(t, me, http.StatusUnauthorized)

	// Structurally valid refresh token signed with the WRONG secret.
	badCfg := &config.Config{
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{
				AccessSecret:  "test-access-secret-do-not-use-in-prod",
				RefreshSecret: "totally-wrong-refresh-secret",
				Issuer:        "go-chi",
				AccessExpiry:  15 * time.Minute,
				RefreshExpiry: 1 * time.Hour,
			},
		},
	}
	badJWT := auth.NewJWT(badCfg)
	wrongSecretToken, err := badJWT.GenerateRefreshToken(lr.UserID, lr.SID, time.Now().Add(time.Hour), nil, nil)
	if err != nil {
		t.Fatalf("generate wrong-secret refresh token: %v", err)
	}
	badRefresh := env.do(http.MethodPost, "/auth/refresh", payloads.RefreshTokenRequest{RefreshToken: wrongSecretToken}, nil)
	requireStatus(t, badRefresh, http.StatusUnauthorized)
	requireCode(t, badRefresh, "E_AUTH_8")
}

// SCENARIO 25 — SESSION METADATA
func TestAuth_SessionMetadata(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "metadata@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	ua := "E2E-Go-Chi-Test/1.0"
	lr := env.login(t, email, "Password123!", ua)

	resp := env.authed(http.MethodGet, "/auth/sessions", lr.Auth.AccessToken, nil)
	requireStatus(t, resp, http.StatusOK)

	var sessions []dto.Session
	decode(t, resp, &sessions)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].UserAgent != ua {
		t.Fatalf("user agent = %q, want %q", sessions[0].UserAgent, ua)
	}
	// httptest requests carry a deterministic RemoteAddr; metadata must come
	// from the transport, not the (nonexistent) login JSON payload.
	if sessions[0].IPAddress != "192.0.2.1" {
		t.Fatalf("ip_address = %q, want 192.0.2.1", sessions[0].IPAddress)
	}
}

// OPTIONAL — malformed login JSON
func TestAuth_LoginMalformedJSON(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	resp := env.doRaw(http.MethodPost, "/auth/login", []byte("{not valid json"), map[string]string{"Content-Type": "application/json"})
	requireStatus(t, resp, http.StatusBadRequest)
}

// OPTIONAL — unknown JSON field rejected
func TestAuth_LoginUnknownField(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	env.createUserFixture(t, "unknown@example.com", "Password123!", enums.StatusActive)

	body := map[string]any{"email": "unknown@example.com", "password": "Password123!", "honeypot": "x"}
	resp := env.do(http.MethodPost, "/auth/login", body, nil)
	requireStatus(t, resp, http.StatusBadRequest)
}

// OPTIONAL — missing required login fields
func TestAuth_LoginMissingFields(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	resp := env.do(http.MethodPost, "/auth/login", map[string]any{"email": ""}, nil)
	requireStatus(t, resp, http.StatusBadRequest)
}

// OPTIONAL — malformed refresh payload
func TestAuth_RefreshMissingField(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	resp := env.do(http.MethodPost, "/auth/refresh", map[string]any{}, nil)
	requireStatus(t, resp, http.StatusBadRequest)
}

// OPTIONAL — duplicate logout
func TestAuth_DuplicateLogout(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "dup-logout@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "")

	first := env.authed(http.MethodPost, "/auth/logout", lr.Auth.AccessToken, nil)
	requireStatus(t, first, http.StatusNoContent)

	second := env.authed(http.MethodPost, "/auth/logout", lr.Auth.AccessToken, nil)
	requireStatus(t, second, http.StatusUnauthorized)
}

// OPTIONAL — duplicate revoke
func TestAuth_DuplicateRevoke(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "dup-revoke@example.com"
	env.createUserFixture(t, email, "Password123!", enums.StatusActive)

	lr := env.login(t, email, "Password123!", "")

	first := env.authed(http.MethodDelete, "/auth/sessions/"+lr.SID, lr.Auth.AccessToken, nil)
	requireStatus(t, first, http.StatusNoContent)

	second := env.authed(http.MethodDelete, "/auth/sessions/"+lr.SID, lr.Auth.AccessToken, nil)
	requireStatus(t, second, http.StatusUnauthorized)
}

// SCENARIO 30 — VERIFY EMAIL VIA GET (DIRECT BROWSER CLICK)
func TestAuth_VerifyEmailViaGET(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	email := "get-verify@example.com"
	userID := env.createUserFixture(t, email, "Password123!", enums.StatusPending)

	rawToken, _ := auth.RandomSessionID()
	tokenHash := auth.HashToken(rawToken)
	now := time.Now()

	err := env.db.Exec(`
		INSERT INTO email_verification_tokens (user_id, token_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?)`,
		userID, tokenHash, now.Add(time.Hour), now,
	).Error
	if err != nil {
		t.Fatalf("insert verification token: %v", err)
	}

	// 1. Verify via GET path /{token}
	resp := env.do(http.MethodGet, "/auth/verify-email/"+rawToken, nil, nil)
	requireStatus(t, resp, http.StatusOK)

	// 2. Confirm user status is now ACTIVE
	var statusID int16
	env.db.Raw("SELECT status_id FROM users WHERE id = ?", userID).Scan(&statusID)
	if statusID != int16(enums.StatusActive) {
		t.Fatalf("expected user to be ACTIVE (2), got %d", statusID)
	}
}
