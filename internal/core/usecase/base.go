package usecase

import (
	"log/slog"

	"github.com/go-playground/validator/v10"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	authusecase "github.com/nassabiq/go-chi-boilerplate/internal/core/usecase/auth"
	storageusecase "github.com/nassabiq/go-chi-boilerplate/internal/core/usecase/storage"
	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/ratelimit"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/storage"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/telemetry"
)

type Dependencies struct {
	Repo          *repositories.BaseRepository
	TxManager     *database.TxManager
	Validator     *validator.Validate
	JWT           *auth.JWT
	Session       *auth.SessionManager
	LoginThrottle *auth.LoginThrottler
	Limiter       *ratelimit.Limiter
	Config        *config.Config
	Publisher     messaging.EventPublisher
	Logger        *slog.Logger
	Metrics       *telemetry.Metrics
	Storage       *storage.S3Storage
}

type BaseUsecase struct {
	*authusecase.AuthUsecase
	*storageusecase.StorageUsecase
	repo          *repositories.BaseRepository
	txManager     *database.TxManager
	validator     *validator.Validate
	jwt           *auth.JWT
	session       *auth.SessionManager
	loginThrottle *auth.LoginThrottler
	limiter       *ratelimit.Limiter
	cfg           *config.Config
	publisher     messaging.EventPublisher
	logger        *slog.Logger
	metrics       *telemetry.Metrics
	storage       *storage.S3Storage
}

func New(deps *Dependencies) *BaseUsecase {
	authUC := authusecase.NewAuthUsecase(&authusecase.AuthDependencies{
		Repo:          deps.Repo,
		TxManager:     deps.TxManager,
		Validator:     deps.Validator,
		JWT:           deps.JWT,
		Session:       deps.Session,
		LoginThrottle: deps.LoginThrottle,
		Limiter:       deps.Limiter,
		Config:        deps.Config,
		Publisher:     deps.Publisher,
		Logger:        deps.Logger,
		Metrics:       deps.Metrics,
	})

	storageUC := storageusecase.NewStorageUsecase(&storageusecase.StorageDependencies{
		Repo:      deps.Repo,
		Validator: deps.Validator,
		Storage:   deps.Storage,
		Config:    deps.Config,
		Logger:    deps.Logger,
	})

	return &BaseUsecase{
		AuthUsecase:    authUC,
		StorageUsecase: storageUC,
		repo:           deps.Repo,
		txManager:      deps.TxManager,
		validator:      deps.Validator,
		jwt:            deps.JWT,
		session:        deps.Session,
		loginThrottle:  deps.LoginThrottle,
		limiter:        deps.Limiter,
		cfg:            deps.Config,
		publisher:      deps.Publisher,
		logger:         deps.Logger,
		metrics:        deps.Metrics,
		storage:        deps.Storage,
	}
}

func (uc *BaseUsecase) bcryptCost() int {
	if uc.cfg != nil && uc.cfg.Auth.Password.BcryptCost > 0 {
		return uc.cfg.Auth.Password.BcryptCost
	}
	return 12
}
