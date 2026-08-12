package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/helper"
	"github.com/redis/go-redis/v9"
)

const (
	failKeyPrefix  = "auth:login:fail:"
	blockKeyPrefix = "auth:login:block:"
)

// failedScript increments the per-account failure counter (expiring after the
// failure window) and, once a threshold is reached, sets a block key whose TTL
// encodes the progressive backoff. It returns the remaining block time in ms
// (0 when the account is not currently throttled).
//
// ARGV layout: failureWindowMs, firstLimit, firstBlockMs, secondLimit,
// secondBlockMs, maxLimit, maxBlockMs.
const failedScript = `
	local failKey = KEYS[1]
	local blockKey = KEYS[2]

	local blockTTL = redis.call("pttl", blockKey)
	if blockTTL > 0 then
		return blockTTL
	end

	local count = redis.call("incr", failKey)
	if count == 1 then
		redis.call("pexpire", failKey, ARGV[1])
	end

	local blockedMs = 0
	if count >= tonumber(ARGV[6]) then
		blockedMs = tonumber(ARGV[7])
	elseif count >= tonumber(ARGV[4]) then
		blockedMs = tonumber(ARGV[5])
	elseif count >= tonumber(ARGV[2]) then
		blockedMs = tonumber(ARGV[3])
	end

	if blockedMs > 0 then
		redis.call("set", blockKey, "1", "px", blockedMs)
	end

	return blockedMs
`

// LoginThrottler enforces progressive account-level failed-login throttling.
type LoginThrottler struct {
	redis *redis.Client
	cfg   config.LoginThrottleConfig
}

// NewLoginThrottler returns a LoginThrottler backed by the given Redis client
// and driven by the provided policy configuration.
func NewLoginThrottler(redis *redis.Client, cfg config.LoginThrottleConfig) *LoginThrottler {
	return &LoginThrottler{redis: redis, cfg: cfg}
}

// Check returns the remaining block duration for the account, or 0 when the
// account is not currently throttled.
func (t *LoginThrottler) Check(ctx context.Context, email string) (time.Duration, error) {
	ttl, err := t.redis.PTTL(ctx, t.blockKey(email)).Result()
	if err != nil {
		return 0, err
	}
	if ttl > 0 {
		return ttl, nil
	}
	return 0, nil
}

// Failed records a failed login attempt and returns the remaining block
// duration if the account crossed a throttle threshold.
func (t *LoginThrottler) Failed(ctx context.Context, email string) (time.Duration, error) {
	failKey := t.failKey(email)
	blockKey := t.blockKey(email)

	args := []any{
		int64(t.cfg.FailureWindow.Milliseconds()),
		int64(t.cfg.FirstLimit), int64(t.cfg.FirstBlock.Milliseconds()),
		int64(t.cfg.SecondLimit), int64(t.cfg.SecondBlock.Milliseconds()),
		int64(t.cfg.MaxLimit), int64(t.cfg.MaxBlock.Milliseconds()),
	}

	res, err := t.redis.Eval(ctx, failedScript, []string{failKey, blockKey}, args...).Result()
	if err != nil {
		return 0, err
	}

	return time.Duration(helper.ToInt64(res)) * time.Millisecond, nil
}

// Success clears any failure and block state for the account.
func (t *LoginThrottler) Success(ctx context.Context, email string) error {
	failKey := t.failKey(email)
	blockKey := t.blockKey(email)

	pipe := t.redis.Pipeline()
	pipe.Del(ctx, failKey)
	pipe.Del(ctx, blockKey)

	_, err := pipe.Exec(ctx)
	return err
}

func (t *LoginThrottler) failKey(email string) string {
	return failKeyPrefix + hashEmail(email)
}

func (t *LoginThrottler) blockKey(email string) string {
	return blockKeyPrefix + hashEmail(email)
}

func hashEmail(email string) string {
	email = helper.NormalizeEmail(email)
	sum := sha256.Sum256([]byte(email))
	return hex.EncodeToString(sum[:])
}
