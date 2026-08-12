package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
)

// CORS handles Cross-Origin Resource Sharing (CORS) based on configuration.
// It securely validates the Origin header, responds to preflight OPTIONS
// requests, and ensures credentials are never paired with wildcard origins.
func CORS(cfg config.CORSConfig) func(http.Handler) http.Handler {
	allowedOrigins := make(map[string]bool, len(cfg.AllowedOrigins))
	allowAll := false
	for _, o := range cfg.AllowedOrigins {
		trimmed := strings.TrimSpace(o)
		if trimmed == "*" {
			allowAll = true
		} else if trimmed != "" {
			allowedOrigins[strings.ToLower(trimmed)] = true
		}
	}

	allowMethods := strings.Join(cfg.AllowedMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowedHeaders, ", ")
	maxAge := strconv.Itoa(cfg.MaxAge)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			matched := false
			if allowAll && !cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				matched = true
			} else if allowedOrigins[strings.ToLower(origin)] {
				// Echo specific origin when matched; never use wildcard with credentials.
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				matched = true
			}

			if !matched {
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			if cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Preflight request handling
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", allowMethods)
				w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
				if cfg.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", maxAge)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
