package auth

import (
	"context"
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

// requestEmailVerification issues a single-use verification token, persists
// only its hash, and publishes the domain event. It never fails registration:
// a publish error is logged and the (already-pending) user can still be
// verified once the mail worker redelivers or the user re-requests.
func (uc *AuthUsecase) requestEmailVerification(ctx context.Context, user *entities.User, meta auth.SessionMeta) {
	rawToken, err := auth.RandomSessionID()
	if err != nil {
		if uc.logger != nil {
			uc.logger.Error("failed to generate email verification token", "error", err, "user_id", user.ID)
		}
		return
	}
	tokenHash := auth.HashToken(rawToken)

	now := time.Now()
	expiresAt := now.Add(uc.cfg.Auth.EmailVerification.TokenTTL)
	verificationURL := helper.BuildResetURL(uc.cfg.Auth.EmailVerification.URL, rawToken)

	token := &entities.EmailVerificationToken{
		UserID:      user.ID,
		TokenHash:   tokenHash,
		ExpiresAt:   expiresAt,
		RequestedIP: helper.NullableString(meta.IPAddress),
		UserAgent:   helper.NullableString(meta.UserAgent),
		CreatedAt:   now,
	}

	if err := uc.txManager.WithTx(ctx, func(ctx context.Context) error {
		if ierr := uc.repo.InvalidatePendingEmailVerificationTokens(ctx, user.ID, now); ierr != nil {
			return ierr
		}
		return uc.repo.CreateEmailVerificationToken(ctx, token)
	}); err != nil {
		if uc.logger != nil {
			uc.logger.Error("failed to persist email verification token", "error", err, "user_id", user.ID)
		}
		return
	}

	eventID, err := auth.RandomSessionID()
	if err != nil {
		if uc.logger != nil {
			uc.logger.Error("failed to generate email verification event id", "error", err, "user_id", user.ID)
		}
		return
	}

	event := messaging.EmailVerificationRequested{
		EventID:         eventID,
		UserID:          user.ID,
		Email:           user.Email,
		Name:            user.Name,
		VerificationURL: verificationURL,
		ExpiresAt:       expiresAt,
		OccurredAt:      now,
	}

	if perr := uc.publisher.PublishEmailVerificationRequested(ctx, event); perr != nil {
		if uc.logger != nil {
			uc.logger.Error("failed to publish email verification requested event", "error", perr, "user_id", user.ID)
		}
	}
}

// VerifyEmail consumes a single-use email verification token and activates the
// account (PENDING -> ACTIVE). It is concurrency-safe: the token row is locked
// for update and marked used inside a transaction, so two concurrent requests
// with the same token cannot both succeed. Only the PENDING -> ACTIVE transition
// is permitted; BLOCKED/SUSPENDED/INACTIVE users are left unchanged and the
// request fails (their status is intentionally not leaked).
func (uc *AuthUsecase) VerifyEmail(ctx context.Context, payload payloads.VerifyEmailRequest) error {
	if err := uc.validator.Struct(payload); err != nil {
		return apperror.New(errconstant.ErrCodeValidation, err)
	}

	tokenHash := auth.HashToken(payload.Token)

	err := uc.txManager.WithTx(ctx, func(ctx context.Context) error {
		token, terr := uc.repo.FindEmailVerificationTokenForUpdate(ctx, tokenHash)
		if terr != nil {
			return terr
		}
		if token.UsedAt.Valid {
			return apperror.New(errconstant.ErrCodeAuthInvalidVerificationToken, nil)
		}
		// Boundary: expiry requires expires_at strictly greater than now.
		if !time.Now().Before(token.ExpiresAt) {
			return apperror.New(errconstant.ErrCodeAuthInvalidVerificationToken, nil)
		}

		found, ferr := uc.repo.FindUserById(ctx, token.UserID)
		if ferr != nil {
			return ferr
		}

		// Only a PENDING account may be activated. An already-ACTIVE account is
		// treated as idempotent success; anything else (BLOCKED/SUSPENDED/
		// INACTIVE) is rejected without leaking which state it is in.
		if found.StatusID != enums.StatusPending && found.StatusID != enums.StatusActive {
			return apperror.New(errconstant.ErrCodeAuthInvalidVerificationToken, nil)
		}

		if merr := uc.repo.MarkEmailVerificationTokenUsed(ctx, token.ID, time.Now()); merr != nil {
			return merr
		}

		// Idempotent for already-active accounts: skip the transition.
		if found.StatusID == enums.StatusPending {
			if aerr := uc.repo.ActivateUser(ctx, found.ID); aerr != nil {
				return aerr
			}
			uc.logAuthActivity(ctx, found.ID, enums.AuthEventEmailVerified, auth.SessionMeta{}, nil)
		}

		return nil
	})
	if err != nil {
		return err
	}

	uc.metrics.RecordEmailVerificationSuccess(ctx)
	return nil
}

// ResendVerification re-issues a verification email for a PENDING account. It
// is enumeration-safe: the response is always identical regardless of whether
// the email is registered or already verified. A per-account rate limit stops
// token flooding and inbox abuse. ACTIVE/BLOCKED/SUSPENDED/INACTIVE accounts
// are silently ignored so their status is never revealed.
func (uc *AuthUsecase) ResendVerification(ctx context.Context, payload payloads.ResendVerificationRequest, meta auth.SessionMeta) error {
	payload.Email = helper.NormalizeEmail(payload.Email)
	if err := uc.validator.Struct(payload); err != nil {
		return apperror.New(errconstant.ErrCodeValidation, err)
	}

	// Account-level rate limit: identical regardless of lookup outcome so it
	// cannot be used to enumerate registered emails either.
	if uc.limiter != nil && uc.cfg != nil {
		key := "auth:resend-verification:" + payload.Email
		allowed, _, err := uc.limiter.Allow(ctx, key, int64(uc.cfg.RateLimit.Resend.Requests), uc.cfg.RateLimit.Resend.Window)
		if err != nil {
			return apperror.New(errconstant.ErrCodeServiceUnavailable, err)
		}
		if !allowed {
			return apperror.New(errconstant.ErrCodeTooManyRequests, nil)
		}
	}

	user, err := uc.repo.FindUserByEmail(ctx, payload.Email)
	if isUserNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	// Only PENDING accounts require (and receive) a fresh verification email.
	if user.StatusID != enums.StatusPending {
		return nil
	}

	uc.logAuthActivity(ctx, user.ID, enums.AuthEventEmailVerified, meta, map[string]any{"action": "resend_verification"})

	// Supersedes any outstanding token and publishes the same domain event as
	// registration. Non-fatal: a later resend can retry.
	uc.requestEmailVerification(ctx, user, meta)
	return nil
}

// RequestEmailChange begins an authenticated email change. It re-authenticates
// with the current password, rejects an already-taken or unchanged address, and
// issues a single-use token sent to the NEW email. The trusted email is NOT
// swapped here; only a confirmed token (VerifyEmailChange) may do that.
func (uc *AuthUsecase) RequestEmailChange(ctx context.Context, userID int64, payload payloads.ChangeEmailRequest, meta auth.SessionMeta) error {
	if err := uc.validator.Struct(payload); err != nil {
		return apperror.New(errconstant.ErrCodeValidation, err)
	}

	newEmail := helper.NormalizeEmail(payload.NewEmail)

	user, err := uc.repo.FindUserById(ctx, userID)
	if err != nil {
		return err
	}

	if err := auth.ComparePassword(user.Password, payload.Password); err != nil {
		return apperror.New(errconstant.ErrCodeAuthInvalidCurrentPassword, err)
	}

	if newEmail == user.Email {
		return apperror.New(errconstant.ErrCodeAuthEmailUnchanged, nil)
	}

	// The new address must not already belong to another account.
	if _, ferr := uc.repo.FindUserByEmail(ctx, newEmail); !isUserNotFound(ferr) {
		if ferr != nil {
			return ferr
		}
		return apperror.New(errconstant.ErrCodeUserAlreadyExists, nil)
	}

	rawToken, err := auth.RandomSessionID()
	if err != nil {
		return apperror.New(errconstant.ErrCodeInternal, err)
	}
	tokenHash := auth.HashToken(rawToken)

	now := time.Now()
	expiresAt := now.Add(uc.cfg.Auth.EmailChange.TokenTTL)
	verificationURL := helper.BuildResetURL(uc.cfg.Auth.EmailChange.URL, rawToken)

	req := &entities.EmailChangeRequest{
		UserID:      user.ID,
		NewEmail:    newEmail,
		TokenHash:   tokenHash,
		ExpiresAt:   expiresAt,
		RequestedIP: helper.NullableString(meta.IPAddress),
		UserAgent:   helper.NullableString(meta.UserAgent),
		CreatedAt:   now,
	}

	if err := uc.txManager.WithTx(ctx, func(ctx context.Context) error {
		if ierr := uc.repo.InvalidatePendingEmailChangeRequests(ctx, user.ID, now); ierr != nil {
			return ierr
		}
		return uc.repo.CreateEmailChangeRequest(ctx, req)
	}); err != nil {
		return err
	}

	eventID, err := auth.RandomSessionID()
	if err != nil {
		if uc.logger != nil {
			uc.logger.Error("failed to generate email change event id", "error", err, "user_id", user.ID)
		}
		return nil
	}

	event := messaging.EmailChangeRequested{
		EventID:         eventID,
		UserID:          user.ID,
		OldEmail:        user.Email,
		NewEmail:        newEmail,
		Name:            user.Name,
		VerificationURL: verificationURL,
		ExpiresAt:       expiresAt,
		OccurredAt:      now,
	}

	if perr := uc.publisher.PublishEmailChangeRequested(ctx, event); perr != nil {
		if uc.logger != nil {
			uc.logger.Error("failed to publish email change requested event", "error", perr, "user_id", user.ID)
		}
	}

	return nil
}

// VerifyEmailChange consumes a single-use email-change token and swaps the
// user's trusted email for the verified NEW address. Concurrency-safe via row
// locking + transaction; two concurrent confirmations cannot both apply.
func (uc *AuthUsecase) VerifyEmailChange(ctx context.Context, payload payloads.VerifyEmailChangeRequest) error {
	if err := uc.validator.Struct(payload); err != nil {
		return apperror.New(errconstant.ErrCodeValidation, err)
	}

	tokenHash := auth.HashToken(payload.Token)

	var userID int64
	err := uc.txManager.WithTx(ctx, func(ctx context.Context) error {
		req, terr := uc.repo.FindEmailChangeTokenForUpdate(ctx, tokenHash)
		if terr != nil {
			return terr
		}
		if req.UsedAt.Valid {
			return apperror.New(errconstant.ErrCodeAuthInvalidEmailChangeToken, nil)
		}
		if !time.Now().Before(req.ExpiresAt) {
			return apperror.New(errconstant.ErrCodeAuthInvalidEmailChangeToken, nil)
		}

		if merr := uc.repo.MarkEmailChangeTokenUsed(ctx, req.ID, time.Now()); merr != nil {
			return merr
		}

		if uerr := uc.repo.UpdateUserEmail(ctx, req.UserID, req.NewEmail); uerr != nil {
			return uerr
		}

		userID = req.UserID
		return nil
	})
	if err != nil {
		return err
	}

	// Security: after the trusted email is swapped, revoke all sessions so
	// the old email's access is fully terminated. The user must re-authenticate
	// with the new email (and password) to get a fresh session.
	if rerr := uc.session.RevokeAll(ctx, userID, uc.jwt.RefreshTTL()); rerr != nil && uc.logger != nil {
		uc.logger.Error("failed to revoke sessions after email change", "error", rerr, "user_id", userID)
	}

	uc.logAuthActivity(ctx, userID, enums.AuthEventEmailChanged, auth.SessionMeta{}, nil)

	return nil
}
