package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/redis/go-redis/v9"
)

var rotateSessionScript = redis.NewScript(`
	local current = redis.call("HGET", KEYS[1], "refresh_hash")
	if not current then
		return 0
	end

	local user_id = redis.call("HGET", KEYS[1], "user_id")
	if not user_id then
		return 0
	end

	if user_id ~= ARGV[3] then
		return 0
	end

	if current ~= ARGV[1] then
		redis.call("DEL", KEYS[1])
		redis.call("SREM", KEYS[2], ARGV[4])
		return -1
	end

	redis.call("HSET", KEYS[1], "refresh_hash", ARGV[2], "rotated_at", ARGV[5])
	return 1
`)

type SessionManager struct {
	redis *redis.Client
}

type Session struct {
	ID          string
	UserID      int64
	RefreshHash string
	IPAddress   string
	UserAgent   string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type SessionMeta struct {
	IPAddress string
	UserAgent string
}

type SessionInfo struct {
	ID        string
	UserID    int64
	IPAddress string
	UserAgent string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func NewSessionManager(redis *redis.Client) *SessionManager {
	return &SessionManager{
		redis: redis,
	}
}

func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func sessionKey(sessionID string) string {
	return "auth:session:" + sessionID
}

func userSessionsKey(userID int64) string {
	return fmt.Sprintf("auth:user:sessions:%d", userID)
}

func (s *SessionManager) List(ctx context.Context, userID int64) ([]SessionInfo, error) {
	indexKey := userSessionsKey(userID)

	sessionIDs, err := s.redis.SMembers(ctx, indexKey).Result()
	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeServiceUnavailable, err)
	}

	if len(sessionIDs) == 0 {
		return []SessionInfo{}, nil
	}

	pipe := s.redis.Pipeline()
	commands := make([]*redis.MapStringStringCmd, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		commands = append(commands, pipe.HGetAll(ctx, sessionKey(sessionID)))
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, apperror.New(errconstant.ErrCodeServiceUnavailable, err)
	}

	sessions := make([]SessionInfo, 0, len(sessionIDs))
	staleSessionIDs := make([]string, 0)

	now := time.Now()

	for i, command := range commands {
		data := command.Val()
		sessionID := sessionIDs[i]

		// Session key sudah expired / hilang,
		// tapi ID masih tertinggal di user index.
		if len(data) == 0 {
			staleSessionIDs = append(staleSessionIDs, sessionID)
			continue
		}

		sessionUserID, err := strconv.ParseInt(data["user_id"], 10, 64)
		if err != nil || sessionUserID != userID {
			staleSessionIDs = append(staleSessionIDs, sessionID)
			continue
		}

		createdAt, err := strconv.ParseInt(data["created_at"], 10, 64)
		if err != nil {
			staleSessionIDs = append(staleSessionIDs, sessionID)
			continue
		}

		expiresAt, err := strconv.ParseInt(data["expires_at"], 10, 64)
		if err != nil {
			staleSessionIDs = append(staleSessionIDs, sessionID)
			continue
		}

		sessionExpiresAt := time.Unix(expiresAt, 0)

		if !sessionExpiresAt.After(now) {
			staleSessionIDs = append(staleSessionIDs, sessionID)
			continue
		}

		sessions = append(
			sessions,
			SessionInfo{
				ID:        sessionID,
				UserID:    sessionUserID,
				IPAddress: data["ip_address"],
				UserAgent: data["user_agent"],
				CreatedAt: time.UnixMilli(createdAt),
				ExpiresAt: sessionExpiresAt,
			},
		)
	}

	// Lazy cleanup stale session ID.
	if len(staleSessionIDs) > 0 {
		members := make([]any, 0, len(staleSessionIDs))
		for _, sessionID := range staleSessionIDs {
			members = append(members, sessionID)
		}

		if err := s.redis.SRem(ctx, indexKey, members...).Err(); err != nil {
			return nil, apperror.New(errconstant.ErrCodeServiceUnavailable, err)
		}
	}

	return sessions, nil
}

func (s *SessionManager) Create(ctx context.Context, session Session) error {
	key := sessionKey(session.ID)
	ttl := time.Until(session.ExpiresAt)
	indexKey := userSessionsKey(session.UserID)

	if ttl <= 0 {
		return apperror.New(errconstant.ErrCodeAuthInvalidToken, nil)
	}

	// The user-session index (auth:user:sessions:<userID>) is shared by every
	// session a user owns. Unconditionally EXPIRE-ing it to the new session's
	// remaining TTL would shorten the index whenever a shorter-lived session is
	// created, which could drop still-valid sessions from the index (breaking
	// List/RevokeAll) once the index expires. Always extend the index to the
	// longest remaining TTL instead of shortening it.
	indexTTL, err := s.redis.PTTL(ctx, indexKey).Result()
	if err != nil {
		return apperror.New(errconstant.ErrCodeServiceUnavailable, err)
	}

	if indexTTL < 0 {
		// -2 (key missing) or -1 (no expiry): adopt the new session TTL.
		indexTTL = 0
	}

	indexExpire := ttl
	if indexTTL > indexExpire {
		indexExpire = indexTTL
	}

	_, err = s.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(
			ctx,
			key,
			"user_id", session.UserID,
			"refresh_hash", session.RefreshHash,
			"ip_address", session.IPAddress,
			"user_agent", session.UserAgent,
			"created_at", session.CreatedAt.UnixMilli(),
			"expires_at", session.ExpiresAt.Unix(),
		)

		pipe.Expire(ctx, key, ttl)
		pipe.SAdd(ctx, indexKey, session.ID)
		pipe.Expire(ctx, indexKey, indexExpire)

		return nil
	})

	if err != nil {
		return apperror.New(errconstant.ErrCodeServiceUnavailable, err)
	}

	return nil
}

func (s *SessionManager) Rotate(ctx context.Context, sessionID string, userID int64, currentHash string, newHash string) error {
	result, err := rotateSessionScript.Run(
		ctx,
		s.redis,
		[]string{
			sessionKey(sessionID),
			userSessionsKey(userID),
		},
		currentHash,
		newHash,
		strconv.FormatInt(userID, 10),
		sessionID,
		strconv.FormatInt(time.Now().UnixMilli(), 10),
	).Int()

	if err != nil {
		return apperror.New(errconstant.ErrCodeServiceUnavailable, err)
	}

	switch result {
	case 1:
		return nil
	case 0:
		return apperror.New(errconstant.ErrCodeAuthInvalidRefreshToken, nil)
	case -1:
		// Refresh-token reuse detected. The Lua script already deleted the
		// session key and removed it from the user-session index atomically,
		// so nothing else is required here.
		return apperror.New(errconstant.ErrCodeAuthInvalidRefreshToken, nil)
	default:
		return apperror.New(errconstant.ErrCodeAuthInvalidRefreshToken, nil)
	}
}

func (s *SessionManager) Validate(ctx context.Context, sessionID string, userID int64) error {
	if sessionID == "" || userID <= 0 {
		return apperror.New(errconstant.ErrCodeAuthInvalidToken, nil)
	}

	session, err := s.redis.HMGet(ctx, sessionKey(sessionID), "user_id", "created_at").Result()
	if err != nil {
		return apperror.New(errconstant.ErrCodeServiceUnavailable, err)
	}

	if len(session) != 2 || session[0] == nil || session[1] == nil {
		return apperror.New(errconstant.ErrCodeAuthInvalidToken, nil)
	}

	sessionUserID, err := strconv.ParseInt(fmt.Sprint(session[0]), 10, 64)
	if err != nil || sessionUserID != userID {
		return apperror.New(errconstant.ErrCodeAuthInvalidToken, err)
	}

	createdAt, err := strconv.ParseInt(fmt.Sprint(session[1]), 10, 64)
	if err != nil {
		return apperror.New(errconstant.ErrCodeAuthInvalidToken, err)
	}

	revokedBefore, err := s.redis.Get(ctx, userRevokedBeforeKey(userID)).Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		return apperror.New(errconstant.ErrCodeServiceUnavailable, err)
	}

	// A session is invalid only if it was created strictly before the
	// logout-all marker. Comparing with "<" (not "<=") ensures a session
	// created in the same millisecond as logout-all is still accepted, so a
	// freshly logged-in user is never locked out by the race between
	// RevokeAll and a new login.
	if err == nil && createdAt < revokedBefore {
		return apperror.New(errconstant.ErrCodeAuthInvalidToken, nil)
	}

	return nil
}

func userRevokedBeforeKey(userID int64) string {
	return fmt.Sprintf("auth:user:revoked_before:%d", userID)
}

func (s *SessionManager) Revoke(ctx context.Context, sessionID string, userID int64) error {
	key := sessionKey(sessionID)

	value, err := s.redis.HGet(ctx, key, "user_id").Int64()
	if errors.Is(err, redis.Nil) {
		return nil
	}

	if err != nil {
		return apperror.New(errconstant.ErrCodeServiceUnavailable, err)
	}

	if value != userID {
		return apperror.New(errconstant.ErrCodeAuthInvalidToken, nil)
	}

	_, err = s.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, sessionKey(sessionID))
		pipe.SRem(ctx, userSessionsKey(userID), sessionID)

		return nil
	})

	if err != nil {
		return apperror.New(errconstant.ErrCodeServiceUnavailable, err)
	}

	return nil
}

func (s *SessionManager) RevokeAll(ctx context.Context, userID int64, ttl time.Duration) error {
	indexKey := userSessionsKey(userID)
	sessionIDs, err := s.redis.SMembers(ctx, indexKey).Result()
	if err != nil {
		return apperror.New(errconstant.ErrCodeServiceUnavailable, err)
	}

	_, err = s.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, sessionID := range sessionIDs {
			pipe.Del(ctx, sessionKey(sessionID))
		}

		pipe.Del(ctx, indexKey)

		pipe.Set(ctx, userRevokedBeforeKey(userID), time.Now().UnixMilli(), ttl)
		return nil
	})

	if err != nil {
		return apperror.New(errconstant.ErrCodeServiceUnavailable, err)
	}

	return nil
}
