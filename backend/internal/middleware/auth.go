package middleware

import (
	"context"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/itchan-dev/itchan/backend/internal/utils"
	jwt_internal "github.com/itchan-dev/itchan/backend/internal/utils/jwt"
	"github.com/itchan-dev/itchan/shared/domain"
)

// Key to store the user claims in the request context
type key int

const userClaimsKey key = 0

func Auth(jwtService *jwt_internal.Jwt, adminOnly bool) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			accessCookie, err := r.Cookie("accessToken")
			if err == http.ErrNoCookie {
				http.Error(w, "Please sign-in", http.StatusUnauthorized)
				return
			} else if err != nil {
				log.Print(err)
				// this error shouldnt happen
				http.Error(w, "Invalid cookie", http.StatusInternalServerError)
				return
			}

			token, err := jwtService.DecodeToken(accessCookie.Value)
			if err != nil {
				utils.WriteErrorAndStatusCode(w, err)
				return
			}

			claims := token.Claims.(jwt.MapClaims)

			if adminOnly {
				isAdmin, ok := claims["admin"].(bool)
				if !ok || !isAdmin {
					http.Error(w, "Access denied. Only for admin", http.StatusForbidden) // 403 Forbidden is more appropriate
					return
				}
			}

			// Create a User struct from the claims
			user := &domain.User{
				Id:    int64(claims["uid"].(float64)),
				Email: claims["email"].(string),
				Admin: claims["admin"].(bool),
			}

			// Store the user in the request context
			ctx := context.WithValue(r.Context(), userClaimsKey, user)
			next(w, r.WithContext(ctx))
		}
	}
}

// Helper functions for admin and regular auth
func AdminOnly(jwtService *jwt_internal.Jwt) func(http.HandlerFunc) http.HandlerFunc {
	return Auth(jwtService, true)
}

func NeedAuth(jwtService *jwt_internal.Jwt) func(http.HandlerFunc) http.HandlerFunc {
	return Auth(jwtService, false)
}

// Function to retrieve the user from the context
func GetUserFromContext(r *http.Request) *domain.User {
	user, ok := r.Context().Value(userClaimsKey).(*domain.User)
	if !ok {
		return nil // Or handle the case where no user is in the context
	}
	return user
}
