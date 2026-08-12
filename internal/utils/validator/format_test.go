package validator_test

import (
	"encoding/json"
	"errors"
	"io"
	"testing"

	govalidator "github.com/go-playground/validator/v10"
	customvalidator "github.com/nassabiq/go-chi-boilerplate/internal/utils/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sampleRequest struct {
	Name     string `json:"name" validate:"required,max=10"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,password"`
}

func TestFormatErrors_ValidationErrors(t *testing.T) {
	v := govalidator.New()
	err := customvalidator.RegisterPasswordValidation(v)
	require.NoError(t, err)

	req := sampleRequest{
		Name:     "ThisNameIsTooLongForMax",
		Email:    "not-an-email",
		Password: "short",
	}

	valErr := v.Struct(req)
	require.Error(t, valErr)

	errs := customvalidator.FormatErrors(valErr)
	require.Len(t, errs, 3)

	assert.Equal(t, "name", errs[0].Field)
	assert.Equal(t, "name must be at most 10 characters", errs[0].Message)

	assert.Equal(t, "email", errs[1].Field)
	assert.Equal(t, "email must be a valid email address", errs[1].Message)

	assert.Equal(t, "password", errs[2].Field)
	assert.Equal(t, "password must be at least 8 characters", errs[2].Message)
}

func TestFormatErrors_PasswordComplexityDetails(t *testing.T) {
	v := govalidator.New()
	err := customvalidator.RegisterPasswordValidation(v)
	require.NoError(t, err)

	// Missing special char
	req := sampleRequest{
		Name:     "Valid",
		Email:    "valid@example.com",
		Password: "Password123",
	}

	valErr := v.Struct(req)
	require.Error(t, valErr)

	errs := customvalidator.FormatErrors(valErr)
	require.Len(t, errs, 1)
	assert.Equal(t, "password", errs[0].Field)
	assert.Equal(t, "password must contain at least one special character", errs[0].Message)
}

func TestFormatErrors_JSONErrors(t *testing.T) {
	t.Run("EOF", func(t *testing.T) {
		errs := customvalidator.FormatErrors(io.EOF)
		require.Len(t, errs, 1)
		assert.Equal(t, "body", errs[0].Field)
		assert.Equal(t, "request body cannot be empty", errs[0].Message)
	})

	t.Run("SyntaxError", func(t *testing.T) {
		var dummy map[string]any
		err := json.Unmarshal([]byte("{bad"), &dummy)
		require.Error(t, err)

		errs := customvalidator.FormatErrors(err)
		require.Len(t, errs, 1)
		assert.Equal(t, "body", errs[0].Field)
		assert.Equal(t, "invalid JSON syntax", errs[0].Message)
	})

	t.Run("UnknownField", func(t *testing.T) {
		err := errors.New("json: unknown field \"extra_field\"")
		errs := customvalidator.FormatErrors(err)
		require.Len(t, errs, 1)
		assert.Equal(t, "extra_field", errs[0].Field)
		assert.Equal(t, "unknown field", errs[0].Message)
	})
}
