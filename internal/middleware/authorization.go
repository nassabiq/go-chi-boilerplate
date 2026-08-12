package middleware

import (
	"net/http"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
)

// RequirePermission creates middleware that checks if the authenticated user has the required permission.
func RequirePermission(permissionCode string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			ctx = payloads.WithRequiredPermission(ctx, permissionCode)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

// RequireAnyPermission creates middleware that checks if the user has at least one of the required permissions.
func RequireAnyPermission(permissionCodes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			ctx = payloads.WithRequiredAnyPermission(ctx, permissionCodes)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

// RequireAllPermissions creates middleware that checks if the user has ALL required permissions.
func RequireAllPermissions(permissionCodes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			ctx = payloads.WithRequiredAllPermissions(ctx, permissionCodes)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

// RequireRole creates middleware that checks if the user has the required role.
func RequireRole(roleCode string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			ctx = payloads.WithRequiredRole(ctx, roleCode)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}