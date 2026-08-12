package router

import (
	"github.com/go-chi/chi/v5"
	appmiddleware "github.com/nassabiq/go-chi-boilerplate/internal/middleware"
)

func registerStorageRoutes(route chi.Router, deps *Dependencies) {
	route.Group(func(route chi.Router) {
		route.Use(appmiddleware.Auth(deps.JWT, deps.Session))

		route.Post("/storage/upload", deps.Handler.UploadFile)
		route.Post("/storage/presign-upload", deps.Handler.GetPresignedUploadURL)
		route.Post("/storage/presign-preview", deps.Handler.GetPresignedPreviewURL)
		route.Get("/storage/presign-preview", deps.Handler.GetPresignedPreviewURL)
		route.Delete("/storage/files", deps.Handler.DeleteFile)

		route.Put("/auth/me/avatar", deps.Handler.UpdateAvatar)
	})
}
