package middleware

import (
	"net/http"
	"strings"

	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/response"
)

func Auth(jwtService *auth.JWT, session *auth.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := strings.TrimSpace(r.Header.Get("Authorization"))
			parts := strings.Fields(header)

			if len(parts) != 2 ||
				!strings.EqualFold(parts[0], "Bearer") {
				response.ErrorResponse(w, apperror.New(errconstant.ErrCodeAuthInvalidToken, nil))
				return
			}

			claims, err := jwtService.ParseAccessToken(parts[1])
			if err != nil {
				response.ErrorResponse(w, err)
				return
			}

			if err := session.Validate(r.Context(), claims.SessionID, claims.UserID); err != nil {
				response.ErrorResponse(w, err)
				return
			}

			ctx := auth.WithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
