package helper

import "strings"

// ToInt64 converts a loosely-typed Redis Lua result into int64. go-redis
// returns Lua numbers as int64, but this also covers int/float64 defensively.
func ToInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
