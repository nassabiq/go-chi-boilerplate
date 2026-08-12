package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/handler"
	appmiddleware "github.com/nassabiq/go-chi-boilerplate/internal/middleware"
)

func registerUserRoutes(route chi.Router, deps *Dependencies) {
	route.Group(func(route chi.Router) {
		route.Use(appmiddleware.Auth(deps.JWT, deps.Session))

		protectedUser(route, deps.Handler)
	})
}

func protectedUser(route chi.Router, handler *handler.BaseHandler) {
	route.Get("/users", handler.FindUsers)
	route.Post("/users", handler.CreateUser)

	route.Get("/users/{id}", handler.FindUserByID)
	route.Put("/users/{id}", handler.UpdateUser)
	route.Delete("/users/{id}", handler.DeleteUser)
	route.Patch("/users/{id}/restore", handler.RestoreUser)
}
