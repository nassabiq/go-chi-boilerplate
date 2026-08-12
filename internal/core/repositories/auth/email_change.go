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

func (r *AuthRepository) CreateEmailChangeRequest(ctx context.Context, req *entities.EmailChangeRequest) error {
	if err := r.DB(ctx).Create(req).Error; err != nil {
		return apperror.New(errconstant.ErrCodeInternal, err)
	}

	return nil
}

// InvalidatePendingEmailChangeRequests supersedes any outstanding (unused,
// unexpired) email-change requests for a user when a fresh one is issued, so an
// old link cannot still swap the address after a newer request.
func (r *AuthRepository) InvalidatePendingEmailChangeRequests(ctx context.Context, userID int64, now time.Time) error {
	result := r.DB(ctx).
		Model(&entities.EmailChangeRequest{}).
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

func (r *AuthRepository) FindEmailChangeTokenForUpdate(ctx context.Context, tokenHash string) (*entities.EmailChangeRequest, error) {
	var req entities.EmailChangeRequest

	err := r.DB(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("token_hash = ?", tokenHash).
		First(&req).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.New(errconstant.ErrCodeAuthInvalidEmailChangeToken, err)
		}
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}

	return &req, nil
}

func (r *AuthRepository) MarkEmailChangeTokenUsed(ctx context.Context, id int64, usedAt time.Time) error {
	result := r.DB(ctx).
		Model(&entities.EmailChangeRequest{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"used_at": usedAt,
		})

	if result.Error != nil {
		return apperror.New(errconstant.ErrCodeInternal, result.Error)
	}

	return nil
}

// UpdateUserEmail sets a new (already-normalized) email for the user. It bumps
// the optimistic-lock version so concurrent edits conflict rather than clobber.
func (r *AuthRepository) UpdateUserEmail(ctx context.Context, userID int64, newEmail string) error {
	result := r.DB(ctx).
		Model(&entities.User{}).
		Where("id = ?", userID).
		Where("deleted_at IS NULL").
		Updates(map[string]any{
			"email":      newEmail,
			"version":    gorm.Expr("version + 1"),
			"updated_at": gorm.Expr("NOW()"),
		})

	if result.Error != nil {
		return apperror.New(errconstant.ErrCodeInternal, result.Error)
	}

	if result.RowsAffected == 0 {
		return apperror.New(errconstant.ErrCodeUserNotFound, nil)
	}

	return nil
}
