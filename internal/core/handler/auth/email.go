package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/dto"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/request"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/response"
)

func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, req *http.Request) {
	var payload payloads.VerifyEmailRequest
	if req.Method == http.MethodGet {
		token := chi.URLParam(req, "token")
		if token == "" {
			token = req.URL.Query().Get("token")
		}
		if token == "" {
			token = req.URL.Query().Get("t")
		}
		payload.Token = token
	} else {
		var err error
		payload, err = request.DecodeJSON[payloads.VerifyEmailRequest](req)
		if err != nil {
			response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
			return
		}
	}

	if err := h.uc.VerifyEmail(req.Context(), payload); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, dto.Message{
		Message: "Email verified. Your account is now active.",
	})
}

func (h *AuthHandler) ResendVerification(w http.ResponseWriter, req *http.Request) {
	payload, err := request.DecodeJSON[payloads.ResendVerificationRequest](req)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	meta := auth.SessionMeta{
		IPAddress: request.ClientIP(req),
		UserAgent: req.UserAgent(),
	}

	// Enumeration-safe: identical response whether or not a verification email
	// was actually sent.
	if err := h.uc.ResendVerification(req.Context(), payload, meta); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, dto.Message{
		Message: "If the email is registered and pending verification, a new verification link has been sent.",
	})
}

func (h *AuthHandler) ChangeEmail(w http.ResponseWriter, req *http.Request) {
	claims, ok := auth.ClaimsFromContext(req.Context())
	if !ok {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil))
		return
	}

	payload, err := request.DecodeJSON[payloads.ChangeEmailRequest](req)
	if err != nil {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
		return
	}

	meta := auth.SessionMeta{
		IPAddress: request.ClientIP(req),
		UserAgent: req.UserAgent(),
	}

	if err := h.uc.RequestEmailChange(req.Context(), claims.UserID, payload, meta); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	// Generic: identical response whether or not a verification email was sent.
	response.Success(w, http.StatusOK, dto.Message{
		Message: "If the new email is valid, a confirmation link has been sent to it.",
	})
}

func (h *AuthHandler) VerifyEmailChange(w http.ResponseWriter, req *http.Request) {
	var payload payloads.VerifyEmailChangeRequest
	if req.Method == http.MethodGet {
		token := chi.URLParam(req, "token")
		if token == "" {
			token = req.URL.Query().Get("token")
		}
		if token == "" {
			token = req.URL.Query().Get("t")
		}
		payload.Token = token
	} else {
		var err error
		payload, err = request.DecodeJSON[payloads.VerifyEmailChangeRequest](req)
		if err != nil {
			response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
			return
		}
	}

	if err := h.uc.VerifyEmailChange(req.Context(), payload); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, dto.Message{
		Message: "Email changed successfully.",
	})
}
