package auth

import (
	"log/slog"

	"github.com/go-playground/validator/v10"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
	pkgauth "github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/ratelimit"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/telemetry"
)

type AuthDependencies struct {
	Repo          *repositories.BaseRepository
	TxManager     *database.TxManager
	Validator     *validator.Validate
	JWT           *pkgauth.JWT
	Session       *pkgauth.SessionManager
	LoginThrottle *pkgauth.LoginThrottler
	Limiter       *ratelimit.Limiter
	Config        *config.Config
	Publisher     messaging.EventPublisher
	Logger        *slog.Logger
	Metrics       *telemetry.Metrics
}

type Dependencies = AuthDependencies

type AuthUsecase struct {
	repo          *repositories.BaseRepository
	txManager     *database.TxManager
	validator     *validator.Validate
	jwt           *pkgauth.JWT
	session       *pkgauth.SessionManager
	loginThrottle *pkgauth.LoginThrottler
	limiter       *ratelimit.Limiter
	cfg           *config.Config
	publisher     messaging.EventPublisher
	logger        *slog.Logger
	metrics       *telemetry.Metrics
}

type Usecase = AuthUsecase

func NewAuthUsecase(deps *AuthDependencies) *AuthUsecase {
	return &AuthUsecase{
		repo:          deps.Repo,
		txManager:     deps.TxManager,
		validator:     deps.Validator,
		jwt:           deps.JWT,
		session:       deps.Session,
		loginThrottle: deps.LoginThrottle,
		limiter:       deps.Limiter,
		cfg:           deps.Config,
		publisher:     deps.Publisher,
		logger:        deps.Logger,
		metrics:       deps.Metrics,
	}
}

func NewUsecase(deps *AuthDependencies) *AuthUsecase {
	return NewAuthUsecase(deps)
}

func (uc *AuthUsecase) bcryptCost() int {
	if uc.cfg != nil && uc.cfg.Auth.Password.BcryptCost > 0 {
		return uc.cfg.Auth.Password.BcryptCost
	}
	return 12
}
