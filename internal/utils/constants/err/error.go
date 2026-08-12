package err

import (
	"fmt"
	"net/http"
)

type ErrorCode string

type ErrorResponse struct {
	Message    string
	HTTPStatus int
}

var errorResponses = map[ErrorCode]ErrorResponse{}

func GetErrorResponse(code ErrorCode) ErrorResponse {
	response, exists := errorResponses[code]
	if exists {
		return response
	}

	return ErrorResponse{
		Message:    "internal server error",
		HTTPStatus: http.StatusInternalServerError,
	}
}

func response(code ErrorCode, message string, httpStatus int) ErrorCode {
	if code == "" {
		panic("error code cannot be empty")
	}

	if message == "" {
		panic(fmt.Sprintf("error message cannot be empty for code %s", code))
	}

	if _, exists := errorResponses[code]; exists {
		panic(fmt.Sprintf("duplicate error code: %s", code))
	}

	errorResponses[code] = ErrorResponse{
		Message:    message,
		HTTPStatus: httpStatus,
	}

	return code
}
