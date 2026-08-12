package auth

import (
	"context"

	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	"gorm.io/gorm"
)

type AuthRepository struct {
	txManager *database.TxManager
}

type Repository = AuthRepository

func NewAuthRepository(txManager *database.TxManager) *AuthRepository {
	return &AuthRepository{
		txManager: txManager,
	}
}

func NewRepository(txManager *database.TxManager) *AuthRepository {
	return NewAuthRepository(txManager)
}

func (r *AuthRepository) DB(ctx context.Context) *gorm.DB {
	return r.txManager.DB(ctx)
}
