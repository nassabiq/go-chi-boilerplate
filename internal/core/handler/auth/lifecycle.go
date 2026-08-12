package auth

import (
	"net/http"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/dto"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/response"
)

func (h *AuthHandler) Deactivate(w http.ResponseWriter, req *http.Request) {
	claims, ok := auth.ClaimsFromContext(req.Context())
	if !ok {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil))
		return
	}

	if err := h.uc.Deactivate(req.Context(), claims.UserID); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, dto.Message{
		Message: "Account deactivated.",
	})
}

func (h *AuthHandler) DeleteAccount(w http.ResponseWriter, req *http.Request) {
	claims, ok := auth.ClaimsFromContext(req.Context())
	if !ok {
		response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil))
		return
	}

	if err := h.uc.DeleteAccount(req.Context(), claims.UserID); err != nil {
		response.ErrorResponse(w, err)
		return
	}

	response.Success(w, http.StatusOK, dto.Message{
		Message: "Account deleted.",
	})
}
