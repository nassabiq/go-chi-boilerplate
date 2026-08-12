package helper

import (
	"database/sql"
	"net/url"
	"strings"
)

// BuildResetURL joins a base URL with a raw token, ensuring exactly one
// slash between them and path-escaping the token. An empty base yields just
// the escaped token.
func BuildResetURL(base, token string) string {
	if base == "" {
		return url.PathEscape(token)
	}
	base = strings.TrimRight(base, "/")
	return base + "/" + url.PathEscape(token)
}

// NullableString wraps a string as sql.NullString, marking it valid only when
// non-empty.
func NullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
