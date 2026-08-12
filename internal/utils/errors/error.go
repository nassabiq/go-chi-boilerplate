package errors

import (
	"fmt"

	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
)

type Error struct {
	Code       errconstant.ErrorCode
	Message    string
	HTTPStatus int
	Err        error
}

func New(code errconstant.ErrorCode, err error) *Error {
	response := errconstant.GetErrorResponse(code)

	return &Error{
		Code:       code,
		Message:    response.Message,
		HTTPStatus: response.HTTPStatus,
		Err:        err,
	}
}

func (e *Error) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}

	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}
