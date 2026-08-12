package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/utils/helper"
	"github.com/redis/go-redis/v9"
)

// allowScript increments a fixed-window counter and only sets the expiry on the
// first increment, so the window boundary is stable. It returns {allowed, ttlMs}.
// allowed is 0 when the limit is exceeded, in which case ttlMs is the remaining
// window time to wait before retrying.
//
// ARGV layout: windowMs (ARGV[1]), limit (ARGV[2]).
const allowScript = `
	local current = redis.call("incr", KEYS[1])
	if current == 1 then
		redis.call("pexpire", KEYS[1], ARGV[1])
	end
	local limit = tonumber(ARGV[2])
	if current > limit then
		local ttl = redis.call("pttl", KEYS[1])
		if ttl < 0 then
			ttl = tonumber(ARGV[1])
		end
		return {0, ttl}
	end
	return {1, 0}
`

// Limiter is a generic fixed-window rate limiter backed by Redis.
type Limiter struct {
	redis *redis.Client
}

// New returns a Redis-backed Limiter.
func New(redis *redis.Client) *Limiter {
	return &Limiter{redis: redis}
}

// Allow reports whether a request identified by key is permitted within the
// given limit/window. When the limit is exceeded it returns the remaining
// window duration as retryAfter. Errors are returned without allowing the
// request so callers can fail closed.
func (l *Limiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (allowed bool, retryAfter time.Duration, err error) {
	res, err := l.redis.Eval(ctx, allowScript, []string{key}, window.Milliseconds(), limit).Result()
	if err != nil {
		return false, 0, err
	}

	arr, ok := res.([]any)
	if !ok || len(arr) != 2 {
		return false, 0, fmt.Errorf("ratelimit: unexpected script result %v", res)
	}

	allowed = helper.ToInt64(arr[0]) == 1
	retryAfter = time.Duration(helper.ToInt64(arr[1])) * time.Millisecond

	return allowed, retryAfter, nil
}
