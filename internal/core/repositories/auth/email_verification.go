package auth

import (
	"context"
	"errors"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *AuthRepository) CreateEmailVerificationToken(ctx context.Context, token *entities.EmailVerificationToken) error {
	if err := r.DB(ctx).Create(token).Error; err != nil {
		return apperror.New(errconstant.ErrCodeInternal, err)
	}

	return nil
}

// InvalidatePendingEmailVerificationTokens marks any outstanding (unused,
// unexpired) verification tokens for a user as used, superseding them when a
// fresh token is issued. This prevents an old link from still activating the
// account after a newer one has been requested.
func (r *AuthRepository) InvalidatePendingEmailVerificationTokens(ctx context.Context, userID int64, now time.Time) error {
	result := r.DB(ctx).
		Model(&entities.EmailVerificationToken{}).
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

func (r *AuthRepository) FindEmailVerificationTokenForUpdate(ctx context.Context, tokenHash string) (*entities.EmailVerificationToken, error) {
	var token entities.EmailVerificationToken

	err := r.DB(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("token_hash = ?", tokenHash).
		First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.New(errconstant.ErrCodeAuthInvalidVerificationToken, err)
		}
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}

	return &token, nil
}

func (r *AuthRepository) MarkEmailVerificationTokenUsed(ctx context.Context, id int64, usedAt time.Time) error {
	result := r.DB(ctx).
		Model(&entities.EmailVerificationToken{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"used_at": usedAt,
		})

	if result.Error != nil {
		return apperror.New(errconstant.ErrCodeInternal, result.Error)
	}

	return nil
}

// ActivateUser transitions a PENDING user to ACTIVE. The conditional
// WHERE status_id = PENDING makes it a safe state transition: a BLOCKED,
// SUSPENDED, INACTIVE or already-ACTIVE user is left untouched (RowsAffected
// == 0). It is invoked inside the same transaction that consumes the
// verification token so the two operations are atomic.
func (r *AuthRepository) ActivateUser(ctx context.Context, userID int64) error {
	result := r.DB(ctx).
		Model(&entities.User{}).
		Where("id = ?", userID).
		Where("deleted_at IS NULL").
		Where("status_id = ?", enums.StatusPending).
		Updates(map[string]any{
			"status_id":  enums.StatusActive,
			"updated_at": gorm.Expr("NOW()"),
			"version":    gorm.Expr("version + 1"),
		})

	if result.Error != nil {
		return apperror.New(errconstant.ErrCodeInternal, result.Error)
	}

	if result.RowsAffected == 0 {
		return apperror.New(errconstant.ErrCodeStaleResource, nil)
	}

	return nil
}
