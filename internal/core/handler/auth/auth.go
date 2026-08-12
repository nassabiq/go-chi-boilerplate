package auth

import (
	"net/http"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/request"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/response"
)

// Login godoc
// @Summary Authenticate user and issue tokens
// @Description Authenticates email and password, returns JWT access and refresh token pair
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body payloads.LoginRequest true "Login credentials"
// @Success 200 {object} dto.Auth
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, req *http.Request) {
	payload, err := request.DecodeJSON[payloads.LoginRequest](req)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	meta := auth.SessionMeta{
		IPAddress: request.ClientIP(req),
		UserAgent: req.UserAgent(),
	}

	result, err := h.uc.Login(req.Context(), payload, meta)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// Register godoc
// @Summary Register a new user
// @Description Registers a new user account with pending status
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body payloads.RegisterRequest true "Registration payload"
// @Success 201 {object} dto.User
// @Failure 400 {object} response.Error
// @Router /auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, req *http.Request) {
	payload, err := request.DecodeJSON[payloads.RegisterRequest](req)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	meta := auth.SessionMeta{
		IPAddress: request.ClientIP(req),
		UserAgent: req.UserAgent(),
	}

	result, err := h.uc.Register(req.Context(), payload, meta)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusCreated, result)
}

// Me godoc
// @Summary Get current user profile
// @Description Returns the profile of the authenticated user
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.User
// @Failure 401 {object} response.Error
// @Router /auth/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, req *http.Request) {
	claims, ok := auth.ClaimsFromContext(req.Context())
	if !ok {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil))
		return
	}

	result, err := h.uc.Me(req.Context(), claims.UserID)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// Refresh godoc
// @Summary Refresh access token
// @Description Rotates refresh token and generates new access token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body payloads.RefreshTokenRequest true "Refresh token payload"
// @Success 200 {object} dto.Auth
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, req *http.Request) {
	payload, err := request.DecodeJSON[payloads.RefreshTokenRequest](req)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	result, err := h.uc.Refresh(req.Context(), payload)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// Logout godoc
// @Summary Logout current session
// @Description Revokes the current session and invalidates refresh token
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Success 204 "No Content"
// @Failure 401 {object} response.Error
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, req *http.Request) {
	claims, ok := auth.ClaimsFromContext(req.Context())
	if !ok {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil))
		return
	}

	if err := h.uc.Logout(req.Context(), claims.UserID, claims.SessionID); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// LogoutAll godoc
// @Summary Logout all active sessions
// @Description Revokes all active sessions for the authenticated user
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Success 204 "No Content"
// @Failure 401 {object} response.Error
// @Router /auth/logout-all [post]
func (h *AuthHandler) LogoutAll(w http.ResponseWriter, req *http.Request) {
	claims, ok := auth.ClaimsFromContext(req.Context())
	if !ok {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil))
		return
	}

	if err := h.uc.LogoutAll(req.Context(), claims.UserID); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// FindSessions godoc
// @Summary List active sessions
// @Description Returns all active sessions for the current authenticated user
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Success 200 {array} dto.Session
// @Failure 401 {object} response.Error
// @Router /auth/sessions [get]
func (h *AuthHandler) FindSessions(w http.ResponseWriter, req *http.Request) {
	claims, ok := auth.ClaimsFromContext(req.Context())
	if !ok {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil))
		return
	}

	result, err := h.uc.FindSessions(req.Context(), claims.UserID, claims.SessionID)
	if err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// RevokeSession godoc
// @Summary Revoke specific session
// @Description Revokes an active session by session ID
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Param session_id path string true "Session ID to revoke"
// @Success 204 "No Content"
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /auth/sessions/{session_id} [delete]
func (h *AuthHandler) RevokeSession(w http.ResponseWriter, req *http.Request) {
	claims, ok := auth.ClaimsFromContext(req.Context())
	if !ok {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil))
		return
	}

	sessionID, err := request.PathParam(req, "session_id")
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthInvalidToken, err))
		return
	}

	if err := h.uc.RevokeSession(req.Context(), claims.UserID, sessionID); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
