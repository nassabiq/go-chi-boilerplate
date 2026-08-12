package validator

import "regexp"

var (
	uppercaseRe   = regexp.MustCompile(`[A-Z]`)
	lowercaseRe   = regexp.MustCompile(`[a-z]`)
	digitRe       = regexp.MustCompile(`[0-9]`)
	specialCharRe = regexp.MustCompile(`[^A-Za-z0-9]`)
)
