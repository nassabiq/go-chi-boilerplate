package auth

import (
	"context"
	"errors"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/dto"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/helper"
)

func (uc *AuthUsecase) logAuthActivity(ctx context.Context, userID int64, eventType enums.AuthEventType, meta auth.SessionMeta, metadata map[string]any) {
	if uc.repo == nil || userID <= 0 {
		return
	}
	if err := uc.repo.LogActivity(ctx, userID, string(eventType), meta.IPAddress, meta.UserAgent, metadata); err != nil && uc.logger != nil {
		uc.logger.Error("failed to log activity", "error", err, "user_id", userID, "event", eventType)
	}
}

func (uc *AuthUsecase) Login(ctx context.Context, payload payloads.LoginRequest, meta auth.SessionMeta) (*dto.Auth, error) {
	payload.Email = helper.NormalizeEmail(payload.Email)
	if err := uc.validator.Struct(payload); err != nil {
		return nil, apperror.New(errconstant.ErrCodeValidation, err)
	}

	retryAfter, err := uc.loginThrottle.Check(ctx, payload.Email)
	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeServiceUnavailable, err)
	}

	if retryAfter > 0 {
		uc.metrics.RecordLoginThrottled(ctx)
		return nil, apperror.New(errconstant.ErrCodeAuthLoginThrottled, nil)
	}

	user, err := uc.repo.FindUserByEmail(ctx, payload.Email)
	if err != nil {
		// Only a missing user should collapse into a generic invalid
		// credential. Any other repository error (DB down, query failure,
		// timeout) propagates so it is classified by its real error code.
		if !isUserNotFound(err) {
			return nil, err
		}

		// Email is not registered: run a bcrypt comparison against a
		// pre-generated dummy hash so this path costs the same as a real
		// wrong-password attempt. Without this, the "user missing" branch is
		// observably cheaper and leaks account existence via timing. The
		// result is intentionally ignored.
		auth.CompareDummyPasswordForCost(payload.Password, uc.bcryptCost())

		if _, ferr := uc.loginThrottle.Failed(ctx, payload.Email); ferr != nil {
			return nil, apperror.New(errconstant.ErrCodeServiceUnavailable, ferr)
		}

		uc.logAuthActivity(ctx, 0, enums.AuthEventLoginFailed, meta, map[string]any{"reason": "user_not_found"})
		uc.metrics.RecordLoginFailure(ctx, "invalid_credentials")
		return nil, apperror.New(errconstant.ErrCodeAuthInvalidCredential, nil)
	}

	// Check Password
	if err := auth.ComparePassword(user.Password, payload.Password); err != nil {
		if _, ferr := uc.loginThrottle.Failed(ctx, payload.Email); ferr != nil {
			return nil, apperror.New(errconstant.ErrCodeServiceUnavailable, ferr)
		}
		uc.logAuthActivity(ctx, user.ID, enums.AuthEventLoginFailed, meta, map[string]any{"reason": "wrong_password"})
		uc.metrics.RecordLoginFailure(ctx, "invalid_credentials")
		return nil, apperror.New(errconstant.ErrCodeAuthInvalidCredential, err)
	}

	if err := uc.loginThrottle.Success(ctx, payload.Email); err != nil {
		return nil, apperror.New(errconstant.ErrCodeServiceUnavailable, err)
	}

	switch user.StatusID {
	case enums.StatusActive:
		// lanjut
	case enums.StatusInactive:
		uc.logAuthActivity(ctx, user.ID, enums.AuthEventLoginFailed, meta, map[string]any{"reason": "inactive"})
		uc.metrics.RecordLoginFailure(ctx, "user_inactive")
		return nil, apperror.New(errconstant.ErrCodeAuthUserInactive, nil)
	case enums.StatusPending:
		uc.logAuthActivity(ctx, user.ID, enums.AuthEventLoginFailed, meta, map[string]any{"reason": "pending"})
		uc.metrics.RecordLoginFailure(ctx, "user_pending")
		return nil, apperror.New(errconstant.ErrCodeAuthUserPending, nil)
	case enums.StatusSuspended:
		uc.logAuthActivity(ctx, user.ID, enums.AuthEventLoginFailed, meta, map[string]any{"reason": "suspended"})
		uc.metrics.RecordLoginFailure(ctx, "user_suspended")
		return nil, apperror.New(errconstant.ErrCodeAuthUserSuspended, nil)
	case enums.StatusBlocked:
		uc.logAuthActivity(ctx, user.ID, enums.AuthEventLoginFailed, meta, map[string]any{"reason": "blocked"})
		uc.metrics.RecordLoginFailure(ctx, "user_blocked")
		return nil, apperror.New(errconstant.ErrCodeAuthUserBlocked, nil)
	default:
		uc.logAuthActivity(ctx, user.ID, enums.AuthEventLoginFailed, meta, map[string]any{"reason": "unknown_status"})
		uc.metrics.RecordLoginFailure(ctx, "user_inactive")
		return nil, apperror.New(errconstant.ErrCodeAuthUserInactive, nil)
	}

	sessionID, err := auth.RandomSessionID()
	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}

	now := time.Now()
	sessionExpiresAt := now.Add(uc.jwt.RefreshTTL())

	// Fetch roles and permissions for the user
	roles, err := uc.repo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	roleCodes := make([]string, len(roles))
	for i, r := range roles {
		roleCodes[i] = r.Code
	}

	permissions, err := uc.repo.GetUserPermissions(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	permissionCodes := make([]string, len(permissions))
	for i, p := range permissions {
		permissionCodes[i] = p.Code
	}

	accessToken, err := uc.jwt.GenerateAccessToken(user.ID, sessionID, roleCodes, permissionCodes)
	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}

	refreshToken, err := uc.jwt.GenerateRefreshToken(user.ID, sessionID, sessionExpiresAt, roleCodes, permissionCodes)
	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}

	// Persist transport metadata so active-session listings surface the
	// device/IP that owns each session. The metadata is supplied by the HTTP
	// handler from the connection, never from trusted JSON request fields.
	session := auth.Session{
		ID:          sessionID,
		UserID:      user.ID,
		RefreshHash: auth.HashToken(refreshToken),
		IPAddress:   meta.IPAddress,
		UserAgent:   meta.UserAgent,
		CreatedAt:   now,
		ExpiresAt:   sessionExpiresAt,
	}

	if err := uc.session.Create(ctx, session); err != nil {
		return nil, err
	}

	// LOGS & TELEMETRY
	uc.logAuthActivity(ctx, user.ID, enums.AuthEventLoginSuccess, meta, nil)
	uc.logAuthActivity(ctx, user.ID, enums.AuthEventSessionCreated, meta, nil)
	uc.metrics.RecordLoginSuccess(ctx)
	uc.metrics.RecordSessionCreated(ctx, "login")

	return &dto.Auth{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    uc.jwt.AccessExpiresIn(),
	}, nil
}

func (uc *AuthUsecase) Register(ctx context.Context, payload payloads.RegisterRequest, meta auth.SessionMeta) (*dto.User, error) {
	payload.Email = helper.NormalizeEmail(payload.Email)
	if err := uc.validator.Struct(payload); err != nil {
		return nil, apperror.New(errconstant.ErrCodeValidation, err)
	}

	hash, err := auth.HashPassword(payload.Password, uc.bcryptCost())
	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}

	user := &entities.User{
		Name:     payload.Name,
		Email:    payload.Email,
		Password: hash,
		StatusID: enums.StatusPending,
		Version:  1,
	}

	// Duplicate email collapses into ErrCodeUserAlreadyExists; the repository
	// maps the unique-violation so the caller never sees the raw DB error.
	if err := uc.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	uc.metrics.RecordRegistrationSuccess(ctx)
	uc.requestEmailVerification(ctx, user, meta)

	return composeUser(user), nil
}

func (uc *AuthUsecase) Refresh(ctx context.Context, payload payloads.RefreshTokenRequest) (*dto.Auth, error) {
	if err := uc.validator.Struct(payload); err != nil {
		return nil, apperror.New(errconstant.ErrCodeValidation, err)
	}

	claims, err := uc.jwt.ParseRefreshToken(payload.RefreshToken)
	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeAuthInvalidRefreshToken, err)
	}

	if claims.SessionID == "" || claims.ExpiresAt == nil {
		return nil, apperror.New(errconstant.ErrCodeAuthInvalidRefreshToken, err)
	}

	if err := uc.session.Validate(ctx, claims.SessionID, claims.UserID); err != nil {
		return nil, err
	}

	accessToken, err := uc.jwt.GenerateAccessToken(claims.UserID, claims.SessionID, claims.Roles, claims.Permissions)
	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}

	refreshToken, err := uc.jwt.GenerateRefreshToken(claims.UserID, claims.SessionID, claims.ExpiresAt.Time, claims.Roles, claims.Permissions)
	if err != nil {
		return nil, apperror.New(errconstant.ErrCodeInternal, err)
	}

	err = uc.session.Rotate(ctx, claims.SessionID, claims.UserID, auth.HashToken(payload.RefreshToken), auth.HashToken(refreshToken))
	if err != nil {
		return nil, err
	}

	uc.metrics.RecordSessionCreated(ctx, "refresh")

	return &dto.Auth{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    uc.jwt.AccessExpiresIn(),
	}, nil
}

func (uc *AuthUsecase) Me(ctx context.Context, userID int64) (*dto.User, error) {
	user, err := uc.repo.FindUserById(ctx, userID)
	if err != nil {
		return nil, err
	}
	return composeUser(user), nil
}

func (uc *AuthUsecase) Logout(ctx context.Context, userID int64, sessionID string) error {
	err := uc.session.Revoke(ctx, sessionID, userID)
	if err == nil {
		uc.logAuthActivity(ctx, userID, enums.AuthEventLogout, auth.SessionMeta{}, nil)
		uc.logAuthActivity(ctx, userID, enums.AuthEventSessionRevoked, auth.SessionMeta{}, map[string]any{"session_id": sessionID})
		uc.metrics.RecordSessionRevoked(ctx, "logout")
	}
	return err
}

func (uc *AuthUsecase) LogoutAll(ctx context.Context, userID int64) error {
	err := uc.session.RevokeAll(ctx, userID, uc.jwt.RefreshTTL())
	if err == nil {
		uc.logAuthActivity(ctx, userID, enums.AuthEventLogoutAll, auth.SessionMeta{}, nil)
		uc.metrics.RecordSessionRevoked(ctx, "logout_all")
	}
	return err
}

func (uc *AuthUsecase) FindSessions(ctx context.Context, userID int64, currentSessionID string) ([]*dto.Session, error) {
	sessions, err := uc.session.List(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]*dto.Session, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, composeSession(session, currentSessionID))
	}

	return result, nil
}

func composeSession(session auth.SessionInfo, currentSessionID string) *dto.Session {
	return &dto.Session{
		ID:        session.ID,
		IPAddress: session.IPAddress,
		UserAgent: session.UserAgent,
		Current:   session.ID == currentSessionID,
		CreatedAt: session.CreatedAt,
		ExpiresAt: session.ExpiresAt,
	}
}

func (uc *AuthUsecase) RevokeSession(ctx context.Context, userID int64, sessionID string) error {
	if sessionID == "" {
		return apperror.New(errconstant.ErrCodeAuthInvalidToken, nil)
	}

	err := uc.session.Revoke(ctx, sessionID, userID)
	if err == nil {
		uc.logAuthActivity(ctx, userID, enums.AuthEventSessionRevoked, auth.SessionMeta{}, map[string]any{"session_id": sessionID})
		uc.metrics.RecordSessionRevoked(ctx, "revoke_single")
	}
	return err
}

func composeUser(user *entities.User) *dto.User {
	return &dto.User{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
		StatusID:  int16(user.StatusID),
		Version:   user.Version,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func isUserNotFound(err error) bool {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code == errconstant.ErrCodeUserNotFound
	}
	return false
}
