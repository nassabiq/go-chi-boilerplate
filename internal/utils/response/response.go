package response

import (
	"encoding/json"
	"errors"
	"net/http"

	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/validator"
)

type Error struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Errors  []validator.FieldError `json:"errors,omitempty"`
}

type ListMeta struct {
	Total int64 `json:"total"`
	Skip  int   `json:"skip"`
	Limit int   `json:"limit"`
}

type List[T any] struct {
	Items T        `json:"items"`
	Meta  ListMeta `json:"meta"`
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}

func Success(w http.ResponseWriter, status int, data any) {
	JSON(w, status, data)
}

func SuccessResponse[T any](w http.ResponseWriter, items T, total int64, skip, limit int) {
	JSON(w, http.StatusOK, List[T]{
		Items: items,
		Meta: ListMeta{
			Total: total,
			Skip:  skip,
			Limit: limit,
		},
	})
}

func ErrorResponse(w http.ResponseWriter, err error) {
	var appErr *apperror.Error

	if !errors.As(err, &appErr) {
		appErr = apperror.New(errconstant.ErrCodeInternal, err)
	}

	errRes := Error{
		Code:    string(appErr.Code),
		Message: appErr.Message,
	}

	if (appErr.Code == errconstant.ErrCodeValidation ||
		appErr.Code == errconstant.ErrCodeInvalidRequest ||
		appErr.Code == errconstant.ErrCodeInvalidParameter) && appErr.Err != nil {
		errRes.Errors = validator.FormatErrors(appErr.Err)
	}

	JSON(w, appErr.HTTPStatus, errRes)
}
