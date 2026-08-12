package auth

import (
	"net/http"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/dto"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/request"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/response"
)

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, req *http.Request) {
	payload, err := request.DecodeJSON[payloads.ForgotPasswordRequest](req)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	meta := auth.SessionMeta{
		IPAddress: request.ClientIP(req),
		UserAgent: req.UserAgent(),
	}

	if err := h.uc.ForgotPassword(req.Context(), payload, meta); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	// Enumeration-safe: identical response regardless of account existence.
	response.Success(w, http.StatusOK, dto.Message{
		Message: "If the email is registered, password reset instructions will be sent.",
	})
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, req *http.Request) {
	payload, err := request.DecodeJSON[payloads.ResetPasswordRequest](req)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	if err := h.uc.ResetPassword(req.Context(), payload); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, dto.Message{
		Message: "Password has been reset.",
	})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, req *http.Request) {
	claims, ok := auth.ClaimsFromContext(req.Context())
	if !ok {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil))
		return
	}

	payload, err := request.DecodeJSON[payloads.ChangePasswordRequest](req)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	if err := h.uc.ChangePassword(req.Context(), claims.UserID, payload); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, dto.Message{
		Message: "Password has been changed.",
	})
}
