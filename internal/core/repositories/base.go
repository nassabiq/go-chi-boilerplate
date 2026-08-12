package repositories

import (
	"context"

	authrepo "github.com/nassabiq/go-chi-boilerplate/internal/core/repositories/auth"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	"gorm.io/gorm"
)

type Dependencies struct {
	TxManager *database.TxManager
}

type BaseRepository struct {
	deps *Dependencies
	*authrepo.AuthRepository
}

func New(deps *Dependencies) *BaseRepository {
	return &BaseRepository{
		deps:           deps,
		AuthRepository: authrepo.NewAuthRepository(deps.TxManager),
	}
}

func (repo *BaseRepository) DB(ctx context.Context) *gorm.DB {
	return repo.deps.TxManager.DB(ctx)
}
