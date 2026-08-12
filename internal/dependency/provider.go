package dependency

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/google/wire"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/handler"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/usecase"
	messaging "github.com/nassabiq/go-chi-boilerplate/internal/messaging"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/cache"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/health"
	natsclient "github.com/nassabiq/go-chi-boilerplate/internal/pkg/nats"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/ratelimit"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/storage"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/telemetry"
	"github.com/nassabiq/go-chi-boilerplate/internal/router"
	customvalidator "github.com/nassabiq/go-chi-boilerplate/internal/utils/validator"
	"gorm.io/gorm"
)

func NewLogger() *slog.Logger {
	return slog.New(
		slog.NewJSONHandler(os.Stdout, nil),
	)
}

func NewValidator() *validator.Validate {
	v := validator.New()
	// Register custom "password" validation tag for password complexity policy.
	if err := customvalidator.RegisterPasswordValidation(v); err != nil {
		panic("failed to register password validator: " + err.Error())
	}
	return v
}

func NewLoginThrottleConfig(cfg *config.Config) config.LoginThrottleConfig {
	return cfg.Auth.LoginThrottle
}

func NewTelemetry() (*telemetry.Metrics, error) {
	provider, err := telemetry.InitTelemetry()
	if err != nil {
		return nil, err
	}
	return provider.Metrics(), nil
}

var ProviderSet = wire.NewSet(
	config.Load,

	database.NewDB,
	database.NewTxManager,

	auth.NewJWT,
	cache.NewRedis,
	ratelimit.New,
	natsclient.New,
	storage.NewS3Storage,
	health.NewChecker,
	wire.Bind(new(messaging.EventPublisher), new(*messaging.Publisher)),
	messaging.NewPublisher,
	auth.NewLoginThrottler,
	auth.NewSessionManager,
	NewLoginThrottleConfig,
	NewTelemetry,
	wire.Struct(new(repositories.Dependencies), "*"),
	repositories.New,

	wire.Struct(new(usecase.Dependencies), "*"),
	usecase.New,

	wire.Struct(new(handler.Dependencies), "*"),
	handler.New,

	wire.Struct(new(router.Dependencies), "*"),
	router.New,
	NewLogger,
	NewValidator,

	wire.Struct(new(App), "*"),
)

type App struct {
	Handler http.Handler
	Config  *config.Config
	DB      *gorm.DB
	Logger  *slog.Logger
	NATS    *natsclient.Client
}
