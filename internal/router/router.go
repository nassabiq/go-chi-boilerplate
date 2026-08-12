package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/handler"
	appmiddleware "github.com/nassabiq/go-chi-boilerplate/internal/middleware"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/ratelimit"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/telemetry"
	_ "github.com/nassabiq/go-chi-boilerplate/docs/swagger"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type Dependencies struct {
	Logger  *slog.Logger
	Handler *handler.BaseHandler
	Limiter *ratelimit.Limiter
	Config  *config.Config
	JWT     *auth.JWT
	Session *auth.SessionManager
}

func New(deps *Dependencies) http.Handler {
	_, _ = telemetry.InitTracer(deps.Config.App.Name)

	route := chi.NewRouter()

	route.Use(chimiddleware.RequestID)
	route.Use(chimiddleware.ClientIPFromRemoteAddr)
	route.Use(appmiddleware.Tracing(deps.Config.App.Name))
	route.Use(appmiddleware.CORS(deps.Config.CORS))
	route.Use(appmiddleware.SecurityHeaders())
	route.Use(appmiddleware.Logger(deps.Logger))
	route.Use(chimiddleware.Recoverer)
	route.Use(chimiddleware.Timeout(30 * time.Second))
	route.Use(chimiddleware.RequestSize(10 << 20)) // 10 MB

	route.Get("/health/live", deps.Handler.Live)
	route.Get("/health/ready", deps.Handler.Ready)
	route.Get("/healthz", deps.Handler.Live)
	route.Get("/swagger/*", httpSwagger.WrapHandler)

	route.With(appmiddleware.BasicAuth(deps.Config.Auth.BasicAuth)).Handle("/metrics", telemetry.PrometheusHandler())

	registerAuthRoutes(route, deps)
	registerUserRoutes(route, deps)
	registerStorageRoutes(route, deps)

	return route
}
