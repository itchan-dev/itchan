package middleware

import (
	"net/http"

	"github.com/itchan-dev/itchan/backend/internal/utils/jwt"
)

func Auth(f http.HandlerFunc, jwt jwt.Jwt, admin_only bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accessCookie, err := r.Cookie("accessToken")
		if err != nil {
			http.Error(w, "please sign-in", http.StatusUnauthorized)
			return
		}
		jwtClaims, err := jwt.DecodeToken(accessCookie.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if admin_only {
			is_admin_claim, ok := jwtClaims["admin"]
			if !ok {
				http.Error(w, "cant get jwt claim", http.StatusInternalServerError)
				return
			}
			is_admin, ok := is_admin_claim.(bool)
			if !ok {
				http.Error(w, "cant typecast jwt claim", http.StatusInternalServerError)
				return
			}
			if !is_admin {
				http.Error(w, "access denied. only for admin", http.StatusUnauthorized)
				return
			}
		}

		f(w, r)
	}
}

func AdminOnly(f http.HandlerFunc, jwt jwt.Jwt) http.HandlerFunc {
	return Auth(f, jwt, true)
}

func NeedAuth(f http.HandlerFunc, jwt jwt.Jwt) http.HandlerFunc {
	return Auth(f, jwt, false)
}
