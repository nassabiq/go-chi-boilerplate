package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/response"
)

// BasicAuth enforces HTTP Basic Authentication using credentials configured in BasicAuthConfig.
// If both Username and Password in the configuration are empty, the middleware is a no-op (bypassed),
// allowing local development and tests to function without mandatory credentials unless enabled.
func BasicAuth(cfg config.BasicAuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Bypass if not configured
			if cfg.Username == "" && cfg.Password == "" {
				next.ServeHTTP(w, r)
				return
			}

			user, pass, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthUnauthorized, nil))
				return
			}

			userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(cfg.Username)) == 1
			passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(cfg.Password)) == 1

			if !userMatch || !passMatch {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthUnauthorized, nil))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
