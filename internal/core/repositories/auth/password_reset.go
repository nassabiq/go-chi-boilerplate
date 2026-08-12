package auth

import (
	"context"
	"errors"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *AuthRepository) CreatePasswordResetToken(ctx context.Context, token *entities.PasswordResetToken) error {
	if err := r.DB(ctx).Create(token).Error; err != nil {
		return apperror.New(errconstant.ErrCodeInternal, err)
	}

	return nil
}

func (r *AuthRepository) FindPasswordResetTokenForUpdate(ctx context.Context, tokenHash string) (*entities.PasswordResetToken, error) {
	var token entities.PasswordResetToken

	err := r.DB(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("token_hash = ?", tokenHash).
		First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.New(errconstant.ErrCodeAuthInvalidResetToken, err)
		}
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}

	return &token, nil
}

func (r *AuthRepository) MarkPasswordResetTokenUsed(ctx context.Context, id int64, usedAt time.Time) error {
	result := r.DB(ctx).
		Model(&entities.PasswordResetToken{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"used_at": usedAt,
		})

	if result.Error != nil {
		return apperror.New(errconstant.ErrCodeInternal, result.Error)
	}

	return nil
}

func (r *AuthRepository) InvalidatePendingPasswordResetTokens(ctx context.Context, userID int64, now time.Time) error {
	result := r.DB(ctx).
		Model(&entities.PasswordResetToken{}).
		Where("user_id = ?", userID).
		Where("used_at IS NULL").
		Updates(map[string]any{
			"used_at": now,
		})

	if result.Error != nil {
		return apperror.New(errconstant.ErrCodeInternal, result.Error)
	}

	return nil
}
