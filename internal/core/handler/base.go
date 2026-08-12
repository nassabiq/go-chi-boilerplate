package handler

import (
	"log/slog"

	authhandler "github.com/nassabiq/go-chi-boilerplate/internal/core/handler/auth"
	healthhandler "github.com/nassabiq/go-chi-boilerplate/internal/core/handler/health"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/usecase"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/health"
)

type Dependencies struct {
	Usecase       *usecase.BaseUsecase
	Logger        *slog.Logger
	HealthChecker *health.Checker
}

type BaseHandler struct {
	*authhandler.AuthHandler
	*healthhandler.HealthHandler
	uc     *usecase.BaseUsecase
	logger *slog.Logger
}

func New(deps *Dependencies) *BaseHandler {
	authH := authhandler.NewAuthHandler(&authhandler.AuthDependencies{
		Usecase: deps.Usecase,
		Logger:  deps.Logger,
	})

	healthH := healthhandler.NewHealthHandler(deps.HealthChecker)

	return &BaseHandler{
		AuthHandler:   authH,
		HealthHandler: healthH,
		uc:            deps.Usecase,
		logger:        deps.Logger,
	}
}

