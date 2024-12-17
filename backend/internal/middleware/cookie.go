package middleware

import (
	"context"
	"net/http"

	"github.com/itchan-dev/itchan/backend/internal/utils/jwt"
)

// Extracts value from cookie to context
func ExtractFromCookie[T any](f http.HandlerFunc, jwt jwt.Jwt, cookieKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accessCookie, err := r.Cookie("accessToken")
		if err != nil {
			http.Error(w, "Please sign-in", http.StatusUnauthorized)
			return
		}
		jwtClaims, err := jwt.DecodeToken(accessCookie.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		claim, ok := jwtClaims[cookieKey]
		if !ok {
			http.Error(w, "Cant get jwt claim", http.StatusInternalServerError)
			return
		}
		value, ok := claim.(T)
		if !ok {
			http.Error(w, "Cant typecast jwt claim", http.StatusInternalServerError)
			return
		}
		ctx := context.WithValue(r.Context(), cookieKey, value)
		f(w, r.WithContext(ctx))
	}
}

func ExtractUserId(f http.HandlerFunc, jwt jwt.Jwt) http.HandlerFunc {
	return ExtractFromCookie[int64](f, jwt, "uid")
}
