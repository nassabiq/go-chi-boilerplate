package auth

import (
	"context"

	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
)

// accountStatusTransitions is the self-service account lifecycle. Only
// transitions an authenticated user can trigger on their own account are
// listed here. SUSPENDED/BLOCKED and their reversal (e.g. reactivate) require
// an admin actor, which is introduced in Phase 14 (RBAC), so they are not
// self-service reachable: an INACTIVE/SUSPENDED/BLOCKED user cannot log in to
// call a protected endpoint in the first place.
var accountStatusTransitions = map[enums.Status]map[enums.Status]bool{
	enums.StatusActive: {enums.StatusInactive: true},
}

// Deactivate moves a logged-in ACTIVE account to INACTIVE and revokes all of
// its sessions. Other current states are rejected with their status error.
func (uc *AuthUsecase) Deactivate(ctx context.Context, userID int64) error {
	user, err := uc.repo.FindUserById(ctx, userID)
	if err != nil {
		return err
	}

	if !accountStatusTransitions[user.StatusID][enums.StatusInactive] {
		return statusError(user.StatusID)
	}

	if err := uc.repo.UpdateUserStatus(ctx, user.ID, enums.StatusInactive); err != nil {
		return err
	}

	uc.logAuthActivity(ctx, user.ID, enums.AuthEventAccountDeactivated, auth.SessionMeta{}, nil)

	// Best-effort: drop every session so the deactivated account cannot be
	// used until it is reactivated (by an admin).
	if rerr := uc.session.RevokeAll(ctx, user.ID, uc.jwt.RefreshTTL()); rerr != nil && uc.logger != nil {
		uc.logger.Error("failed to revoke sessions after deactivate", "error", rerr, "user_id", user.ID)
	}

	return nil
}

// DeleteAccount soft-deletes the authenticated user's own account and revokes
// all sessions. Only an ACTIVE account holds a usable session, so deletion is
// gated on ACTIVE.
func (uc *AuthUsecase) DeleteAccount(ctx context.Context, userID int64) error {
	user, err := uc.repo.FindUserById(ctx, userID)
	if err != nil {
		return err
	}

	if user.StatusID != enums.StatusActive {
		return statusError(user.StatusID)
	}

	if err := uc.repo.DeleteUser(ctx, user.ID, user.Version); err != nil {
		return err
	}

	uc.logAuthActivity(ctx, user.ID, enums.AuthEventAccountDeleted, auth.SessionMeta{}, nil)

	if rerr := uc.session.RevokeAll(ctx, user.ID, uc.jwt.RefreshTTL()); rerr != nil && uc.logger != nil {
		uc.logger.Error("failed to revoke sessions after account deletion", "error", rerr, "user_id", user.ID)
	}

	return nil
}

// statusError maps a non-ACTIVE current status to its public status error so
// lifecycle operations fail with a consistent, non-leaking message.
func statusError(status enums.Status) error {
	switch status {
	case enums.StatusInactive:
		return apperror.New(errconstant.ErrCodeAuthUserInactive, nil)
	case enums.StatusSuspended:
		return apperror.New(errconstant.ErrCodeAuthUserSuspended, nil)
	case enums.StatusBlocked:
		return apperror.New(errconstant.ErrCodeAuthUserBlocked, nil)
	default:
		return apperror.New(errconstant.ErrCodeAuthUserInactive, nil)
	}
}
