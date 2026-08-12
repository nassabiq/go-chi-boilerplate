package middleware

import (
	"math"
	"net/http"
	"strconv"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/ratelimit"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/response"
)

const loginIPKeyPrefix = "auth:login:ip:"

// LoginRateLimit limits login attempts per client IP using a Redis-backed
// fixed-window limiter. It relies on chi's ClientIP middleware for the trusted
// client IP, so it does not parse X-Forwarded-For itself. When rate limiting is
// disabled via cfg.RateLimit.Enabled it passes the request through untouched.
func LoginRateLimit(limiter *ratelimit.Limiter, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !cfg.RateLimit.Enabled {
				next.ServeHTTP(w, req)
				return
			}

			rule := cfg.RateLimit.Login
			ip := chimiddleware.GetClientIP(req.Context())
			key := loginIPKeyPrefix + ip

			allowed, retryAfter, err := limiter.Allow(req.Context(), key, int64(rule.Requests), rule.Window)
			if err != nil {
				response.ErrorResponse(w, apperror.New(errconstant.ErrCodeServiceUnavailable, err))
				return
			}

			if !allowed {
				w.Header().Set("Retry-After", retryAfterSeconds(retryAfter))
				response.ErrorResponse(w, apperror.New(errconstant.ErrCodeTooManyRequests, nil))
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

// retryAfterSeconds returns the Retry-After value in whole seconds, rounded up
// so clients never retry too early, with a 1-second floor for any positive
// remaining duration.
func retryAfterSeconds(d time.Duration) string {
	seconds := int64(math.Ceil(d.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return strconv.FormatInt(seconds, 10)
}
