package auth

import (
	"context"
	"errors"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/helper"
)

func (uc *AuthUsecase) ForgotPassword(ctx context.Context, payload payloads.ForgotPasswordRequest, meta auth.SessionMeta) error {
	payload.Email = helper.NormalizeEmail(payload.Email)
	if err := uc.validator.Struct(payload); err != nil {
		return apperror.New(errconstant.ErrCodeValidation, err)
	}

	// Per-account rate limit applied before the lookup so it cannot be used to
	// enumerate registered emails either.
	if uc.limiter != nil && uc.cfg != nil {
		key := "auth:forgot-password:" + payload.Email
		allowed, _, err := uc.limiter.Allow(ctx, key, int64(uc.cfg.RateLimit.Forgot.Requests), uc.cfg.RateLimit.Forgot.Window)
		if err != nil {
			return apperror.New(errconstant.ErrCodeServiceUnavailable, err)
		}
		if !allowed {
			return apperror.New(errconstant.ErrCodeTooManyRequests, nil)
		}
	}

	user, err := uc.repo.FindUserByEmail(ctx, payload.Email)
	if isUserNotFound(err) {
		// Unknown email: same generic outcome, nothing is published.
		return nil
	}
	if err != nil {
		return err
	}

	rawToken, err := auth.RandomSessionID()
	if err != nil {
		return apperror.New(errconstant.ErrCodeInternal, err)
	}
	tokenHash := auth.HashToken(rawToken)

	now := time.Now()
	expiresAt := now.Add(uc.cfg.Auth.PasswordReset.TokenTTL)
	resetURL := helper.BuildResetURL(uc.cfg.Auth.PasswordReset.URL, rawToken)

	token := &entities.PasswordResetToken{
		UserID:      user.ID,
		TokenHash:   tokenHash,
		ExpiresAt:   expiresAt,
		RequestedIP: helper.NullableString(meta.IPAddress),
		UserAgent:   helper.NullableString(meta.UserAgent),
		CreatedAt:   now,
	}

	if err := uc.txManager.WithTx(ctx, func(ctx context.Context) error {
		if ierr := uc.repo.InvalidatePendingPasswordResetTokens(ctx, user.ID, now); ierr != nil {
			return ierr
		}
		return uc.repo.CreatePasswordResetToken(ctx, token)
	}); err != nil {
		return err
	}

	eventID, err := auth.RandomSessionID()
	if err != nil {
		if uc.logger != nil {
			uc.logger.Error("failed to generate password reset event id", "error", err, "user_id", user.ID)
		}
		return nil
	}

	event := messaging.PasswordResetRequested{
		EventID:    eventID,
		UserID:     user.ID,
		Email:      user.Email,
		Name:       user.Name,
		ResetURL:   resetURL,
		ExpiresAt:  expiresAt,
		OccurredAt: now,
	}

	if perr := uc.publisher.PublishPasswordResetRequested(ctx, event); perr != nil {
		if uc.logger != nil {
			uc.logger.Error("failed to publish password reset requested event", "error", perr, "user_id", user.ID)
		}
	}

	uc.metrics.RecordPasswordResetRequested(ctx)
	return nil
}

func (uc *AuthUsecase) ResetPassword(ctx context.Context, payload payloads.ResetPasswordRequest) error {
	if err := uc.validator.Struct(payload); err != nil {
		return apperror.New(errconstant.ErrCodeValidation, err)
	}

	tokenHash := auth.HashToken(payload.Token)
	var user *entities.User

	err := uc.txManager.WithTx(ctx, func(ctx context.Context) error {
		token, terr := uc.repo.FindPasswordResetTokenForUpdate(ctx, tokenHash)
		if terr != nil {
			return terr
		}
		if token.UsedAt.Valid {
			return apperror.New(errconstant.ErrCodeAuthInvalidResetToken, nil)
		}
		// Boundary: a token whose expires_at equals now is expired. Validity
		// requires expires_at strictly greater than the current time.
		if !time.Now().Before(token.ExpiresAt) {
			return apperror.New(errconstant.ErrCodeAuthInvalidResetToken, nil)
		}

		found, ferr := uc.repo.FindUserById(ctx, token.UserID)
		if ferr != nil {
			return ferr
		}

		// Reject reusing the current password.
		if cmpErr := auth.ComparePassword(found.Password, payload.NewPassword); cmpErr == nil {
			return apperror.New(errconstant.ErrCodeAuthSamePassword, nil)
		}

		hash, herr := auth.HashPassword(payload.NewPassword, uc.bcryptCost())
		if herr != nil {
			return apperror.New(errconstant.ErrCodeInternal, herr)
		}

		if uerr := uc.repo.UpdateUserPassword(ctx, found.ID, hash); uerr != nil {
			return uerr
		}

		if merr := uc.repo.MarkPasswordResetTokenUsed(ctx, token.ID, time.Now()); merr != nil {
			return merr
		}

		user = found
		return nil
	})
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			switch appErr.Code {
			case errconstant.ErrCodeAuthInvalidResetToken:
				uc.metrics.RecordPasswordResetFailure(ctx, "invalid_token")
			case errconstant.ErrCodeAuthSamePassword:
				uc.metrics.RecordPasswordResetFailure(ctx, "same_password")
			default:
				uc.metrics.RecordPasswordResetFailure(ctx, "error")
			}
		} else {
			uc.metrics.RecordPasswordResetFailure(ctx, "error")
		}
		return err
	}

	if rerr := uc.session.RevokeAll(ctx, user.ID, uc.jwt.RefreshTTL()); rerr != nil {
		if uc.logger != nil {
			uc.logger.Error("failed to revoke sessions after password reset", "error", rerr, "user_id", user.ID)
		}
	}

	uc.metrics.RecordPasswordResetSuccess(ctx)
	uc.metrics.RecordSessionRevoked(ctx, "password_reset")
	uc.logAuthActivity(ctx, user.ID, enums.AuthEventPasswordReset, auth.SessionMeta{}, nil)

	// Security notification: a successful reset must notify the user even if
	// they triggered it themselves (account-takeover signal).
	eventID, eerr := auth.RandomSessionID()
	if eerr == nil {
		event := messaging.PasswordChanged{
			EventID:    eventID,
			UserID:     user.ID,
			Email:      user.Email,
			Name:       user.Name,
			Reason:     "reset_password",
			OccurredAt: time.Now(),
		}
		if perr := uc.publisher.PublishPasswordChanged(ctx, event); perr != nil && uc.logger != nil {
			uc.logger.Error("failed to publish password changed event", "error", perr, "user_id", user.ID)
		}
	}

	return nil
}

func (uc *AuthUsecase) ChangePassword(ctx context.Context, userID int64, payload payloads.ChangePasswordRequest) error {
	if err := uc.validator.Struct(payload); err != nil {
		return apperror.New(errconstant.ErrCodeValidation, err)
	}

	user, err := uc.repo.FindUserById(ctx, userID)
	if err != nil {
		return err
	}

	if err := auth.ComparePassword(user.Password, payload.CurrentPassword); err != nil {
		return apperror.New(errconstant.ErrCodeAuthInvalidCurrentPassword, err)
	}

	// Reject reusing the current password.
	if cmpErr := auth.ComparePassword(user.Password, payload.NewPassword); cmpErr == nil {
		return apperror.New(errconstant.ErrCodeAuthSamePassword, nil)
	}

	hash, err := auth.HashPassword(payload.NewPassword, uc.bcryptCost())
	if err != nil {
		return apperror.New(errconstant.ErrCodeInternal, err)
	}

	if err := uc.repo.UpdateUserPassword(ctx, user.ID, hash); err != nil {
		return err
	}

	// Revoke every session after a password change: a changed credential
	// invalidates all existing refresh tokens. Safe default; preserving the
	// current session would require an explicit security policy.
	if rerr := uc.session.RevokeAll(ctx, user.ID, uc.jwt.RefreshTTL()); rerr != nil {
		if uc.logger != nil {
			uc.logger.Error("failed to revoke sessions after password change", "error", rerr, "user_id", user.ID)
		}
	}

	uc.metrics.RecordSessionRevoked(ctx, "password_change")
	uc.logAuthActivity(ctx, user.ID, enums.AuthEventPasswordChanged, auth.SessionMeta{}, nil)

	eventID, eerr := auth.RandomSessionID()
	if eerr == nil {
		event := messaging.PasswordChanged{
			EventID:    eventID,
			UserID:     user.ID,
			Email:      user.Email,
			Name:       user.Name,
			Reason:     "change_password",
			OccurredAt: time.Now(),
		}

		if perr := uc.publisher.PublishPasswordChanged(ctx, event); perr != nil && uc.logger != nil {
			uc.logger.Error("failed to publish password changed event", "error", perr, "user_id", user.ID)
		}
	}

	return nil
}
