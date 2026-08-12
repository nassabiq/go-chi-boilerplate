package auth

import (
	"log/slog"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/usecase"
)

type AuthDependencies struct {
	Usecase *usecase.BaseUsecase
	Logger  *slog.Logger
}

type Dependencies = AuthDependencies

type AuthHandler struct {
	uc     *usecase.BaseUsecase
	logger *slog.Logger
}

type Handler = AuthHandler

func NewAuthHandler(deps *AuthDependencies) *AuthHandler {
	return &AuthHandler{
		uc:     deps.Usecase,
		logger: deps.Logger,
	}
}

func NewHandler(deps *AuthDependencies) *AuthHandler {
	return NewAuthHandler(deps)
}
