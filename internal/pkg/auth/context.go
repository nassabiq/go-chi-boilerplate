package auth

import "context"

type claimsContextKey struct{}

func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, claims)
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(*Claims)
	return claims, ok
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return 0, false
	}

	return claims.UserID, true
}
