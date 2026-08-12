package response_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	govalidator "github.com/go-playground/validator/v10"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/response"
	customvalidator "github.com/nassabiq/go-chi-boilerplate/internal/utils/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type userReq struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

func TestErrorResponse_ValidationError(t *testing.T) {
	v := govalidator.New()
	_ = customvalidator.RegisterPasswordValidation(v)

	req := userReq{Name: "", Email: "bad"}
	valErr := v.Struct(req)
	require.Error(t, valErr)

	appErr := apperror.New(errconstant.ErrCodeValidation, valErr)

	w := httptest.NewRecorder()
	response.ErrorResponse(w, appErr)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var res response.Error
	err := json.Unmarshal(w.Body.Bytes(), &res)
	require.NoError(t, err)

	assert.Equal(t, "E_VAL_1", res.Code)
	assert.Equal(t, "validation failed", res.Message)
	require.Len(t, res.Errors, 2)

	assert.Equal(t, "name", res.Errors[0].Field)
	assert.Equal(t, "name is required", res.Errors[0].Message)

	assert.Equal(t, "email", res.Errors[1].Field)
	assert.Equal(t, "email must be a valid email address", res.Errors[1].Message)
}

func TestErrorResponse_NonValidationError_NoErrorsField(t *testing.T) {
	appErr := apperror.New(errconstant.ErrCodeAuthInvalidCredential, errors.New("internal detail"))

	w := httptest.NewRecorder()
	response.ErrorResponse(w, appErr)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var raw map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &raw)
	require.NoError(t, err)

	assert.Equal(t, "E_AUTH_1", raw["code"])
	assert.Equal(t, "invalid email or password", raw["message"])
	assert.Nil(t, raw["errors"], "errors field must not be present in non-validation errors")
}
