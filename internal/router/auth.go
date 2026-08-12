package router

import (
	"github.com/go-chi/chi/v5"
	appmiddleware "github.com/nassabiq/go-chi-boilerplate/internal/middleware"
)

func registerAuthRoutes(route chi.Router, deps *Dependencies) {
	route.Group(func(route chi.Router) {
		route.Use(appmiddleware.BasicAuth(deps.Config.Auth.BasicAuth))
		publicAuth(route, deps)
	})

	route.Group(func(route chi.Router) {
		route.Use(appmiddleware.Auth(deps.JWT, deps.Session))
		protectedAuth(route, deps)
	})
}

func publicAuth(route chi.Router, deps *Dependencies) {
	route.Post("/auth/refresh", deps.Handler.Refresh)
	route.With(appmiddleware.LoginRateLimit(deps.Limiter, deps.Config)).
		Post("/auth/login", deps.Handler.Login)
	route.Post("/auth/register", deps.Handler.Register)

	// Email Verification (Supports POST with JSON, GET with ?token=, and GET /{token})
	route.Post("/auth/verify-email", deps.Handler.VerifyEmail)
	route.Get("/auth/verify-email", deps.Handler.VerifyEmail)
	route.Get("/auth/verify-email/{token}", deps.Handler.VerifyEmail)

	// Email Change Confirmation (Supports POST with JSON, GET with ?token=, and GET /{token})
	route.Post("/auth/verify-email-change", deps.Handler.VerifyEmailChange)
	route.Get("/auth/verify-email-change", deps.Handler.VerifyEmailChange)
	route.Get("/auth/verify-email-change/{token}", deps.Handler.VerifyEmailChange)

	route.Post("/auth/resend-verification", deps.Handler.ResendVerification)
	route.Post("/auth/forgot-password", deps.Handler.ForgotPassword)
	route.Post("/auth/reset-password", deps.Handler.ResetPassword)
}

func protectedAuth(route chi.Router, deps *Dependencies) {
	route.Get("/auth/me", deps.Handler.Me)
	route.Post("/auth/logout", deps.Handler.Logout)
	route.Post("/auth/logout-all", deps.Handler.LogoutAll)
	route.Get("/auth/sessions", deps.Handler.FindSessions)
	route.Delete("/auth/sessions/{session_id}", deps.Handler.RevokeSession)
	route.Post("/auth/change-password", deps.Handler.ChangePassword)
	route.Post("/auth/change-email", deps.Handler.ChangeEmail)
	route.Post("/auth/deactivate", deps.Handler.Deactivate)
	route.Delete("/auth/account", deps.Handler.DeleteAccount)

	// Role management
	route.Post("/roles", deps.Handler.CreateRole)
	route.Get("/roles/{code}", deps.Handler.GetRoleByCode)
	route.Get("/roles/{code}/permissions", deps.Handler.GetRolePermissions)

	// Permission management
	route.Post("/permissions", deps.Handler.CreatePermission)
	route.Get("/permissions/{code}", deps.Handler.GetPermissionByCode)

	// Role-Permission assignment
	route.Post("/roles/{code}/permissions", deps.Handler.AssignPermissionToRole)

	// User-Role assignment
	route.Post("/users/{user_id}/roles", deps.Handler.AssignRoleToUser)
	route.Get("/users/{user_id}/roles", deps.Handler.GetUserRoles)
	route.Get("/users/{user_id}/permissions", deps.Handler.GetUserPermissions)
	route.Delete("/users/{user_id}/roles/{code}", deps.Handler.RemoveRoleFromUser)

	// Permission check
	route.Get("/users/{user_id}/permissions/check", deps.Handler.CheckUserPermission)
}
