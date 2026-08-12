package validator

import "errors"

var (
	errPasswordTooShort    = errors.New("password too short")
	errPasswordTooLong     = errors.New("password too long")
	errPasswordNoUppercase = errors.New("password missing uppercase")
	errPasswordNoLowercase = errors.New("password missing lowercase")
	errPasswordNoDigit     = errors.New("password missing digit")
	errPasswordNoSpecial   = errors.New("password missing special character")
)
