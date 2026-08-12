package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// FormatErrors transforms a validation or decode error into a slice of FieldError.
func FormatErrors(err error) []FieldError {
	if err == nil {
		return nil
	}

	var valErrs validator.ValidationErrors
	if errors.As(err, &valErrs) {
		res := make([]FieldError, 0, len(valErrs))
		for _, fe := range valErrs {
			field := fe.Field()
			if field == "" {
				field = fe.StructField()
			}
			field = formatFieldName(field)
			res = append(res, FieldError{
				Field:   field,
				Message: formatFieldErrorMessage(fe, field),
			})
		}
		return res
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if field == "" {
			field = "body"
		}
		return []FieldError{
			{
				Field:   formatFieldName(field),
				Message: fmt.Sprintf("expected %s, got %s", typeErr.Type.String(), typeErr.Value),
			},
		}
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return []FieldError{
			{
				Field:   "body",
				Message: "invalid JSON syntax",
			},
		}
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return []FieldError{
			{
				Field:   "body",
				Message: "request body cannot be empty",
			},
		}
	}

	errStr := err.Error()
	if strings.Contains(errStr, "unknown field") {
		parts := strings.Split(errStr, "\"")
		field := "body"
		if len(parts) >= 2 {
			field = parts[1]
		}
		return []FieldError{
			{
				Field:   field,
				Message: "unknown field",
			},
		}
	}

	return []FieldError{
		{
			Field:   "body",
			Message: errStr,
		},
	}
}

func formatFieldName(field string) string {
	if field == "" {
		return "field"
	}

	hasUpper := false
	for _, r := range field {
		if unicode.IsUpper(r) {
			hasUpper = true
			break
		}
	}
	if !hasUpper {
		return field
	}

	var b strings.Builder
	for i, r := range field {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteRune('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func formatFieldErrorMessage(fe validator.FieldError, field string) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "password":
		if valStr, ok := fe.Value().(string); ok {
			if pErr := DefaultPasswordPolicy().Validate(valStr); pErr != nil {
				return PasswordValidationError(pErr)
			}
		}
		return fmt.Sprintf("%s does not meet password requirements", field)
	case "min":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
		}
		return fmt.Sprintf("%s must be at least %s", field, fe.Param())
	case "max":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
		}
		return fmt.Sprintf("%s must be at most %s", field, fe.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, fe.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s]", field, fe.Param())
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", field)
	default:
		return fmt.Sprintf("%s failed %s validation", field, fe.Tag())
	}
}
