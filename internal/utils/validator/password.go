package validator

import (
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"
)

// PasswordPolicy defines the minimum complexity requirements for passwords.
type PasswordPolicy struct {
	MinLength int
	MaxLength int
}

// DefaultPasswordPolicy returns a policy matching common OWASP guidance.
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength: 8,
		MaxLength: 72, // bcrypt truncation limit
	}
}

// Validate checks a password against the policy. Returns nil if valid.
func (p PasswordPolicy) Validate(password string) error {
	if utf8.RuneCountInString(password) < p.MinLength {
		return errPasswordTooShort
	}
	if utf8.RuneCountInString(password) > p.MaxLength {
		return errPasswordTooLong
	}
	if !uppercaseRe.MatchString(password) {
		return errPasswordNoUppercase
	}
	if !lowercaseRe.MatchString(password) {
		return errPasswordNoLowercase
	}
	if !digitRe.MatchString(password) {
		return errPasswordNoDigit
	}
	if !specialCharRe.MatchString(password) {
		return errPasswordNoSpecial
	}
	return nil
}

// passwordValidator is a reusable validator.Func compatible with the
// go-playground/validator RegisterValidation API. It enforces the default
// password policy (8–72 chars, upper, lower, digit, special).
func passwordValidator(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	return DefaultPasswordPolicy().Validate(password) == nil
}

// RegisterPasswordValidation registers the "password" custom validation tag
// and json tag name resolver on the given validator.
func RegisterPasswordValidation(v *validator.Validate) error {
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
	return v.RegisterValidation("password", passwordValidator)
}

// PasswordValidationError returns a descriptive error message for a password
// validation failure.
func PasswordValidationError(err error) string {
	switch err {
	case errPasswordTooShort:
		return "password must be at least " + strconv.Itoa(DefaultPasswordPolicy().MinLength) + " characters"
	case errPasswordTooLong:
		return "password must be at most " + strconv.Itoa(DefaultPasswordPolicy().MaxLength) + " characters"
	case errPasswordNoUppercase:
		return "password must contain at least one uppercase letter"
	case errPasswordNoLowercase:
		return "password must contain at least one lowercase letter"
	case errPasswordNoDigit:
		return "password must contain at least one digit"
	case errPasswordNoSpecial:
		return "password must contain at least one special character"
	default:
		return err.Error()
	}
}
