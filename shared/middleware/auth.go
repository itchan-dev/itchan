package middleware

import (
	"context"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/itchan-dev/itchan/shared/domain"
	jwt_internal "github.com/itchan-dev/itchan/shared/jwt"
	"github.com/itchan-dev/itchan/shared/utils"
)

// BlacklistCache interface defines methods needed by auth middleware
type BlacklistCache interface {
	IsBlacklisted(userId domain.UserId) bool
}

// Key to store the user claims in the request context
type key int

const UserClaimsKey key = 0

func Auth(jwtService jwt_internal.JwtService, blacklistCache BlacklistCache, secureCookies bool, adminOnly bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				log.Print("Invalid JWT claims format")
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Extract and validate required claims
			uidFloat, ok := claims["uid"].(float64)
			if !ok {
				log.Print("Missing or invalid uid claim in JWT")
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			email, ok := claims["email"].(string)
			if !ok {
				log.Print("Missing or invalid email claim in JWT")
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			isAdmin, ok := claims["admin"].(bool)
			if !ok {
				log.Print("Missing or invalid admin claim in JWT")
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			if adminOnly && !isAdmin {
				http.Error(w, "Access denied. Only for admin", http.StatusForbidden)
				return
			}

			// Create a User struct from the claims
			user := &domain.User{
				Id:    int64(uidFloat),
				Email: email,
				Admin: isAdmin,
			}

			// Check if user is blacklisted
			if blacklistCache != nil && blacklistCache.IsBlacklisted(user.Id) {
				// Clear JWT cookie to force re-login
				cookie := &http.Cookie{
					Path:     "/",
					Name:     "accessToken",
					Value:    "",
					MaxAge:   -1,
					HttpOnly: true,
					Secure:   secureCookies,
					SameSite: http.SameSiteLaxMode,
				}
				http.SetCookie(w, cookie)
				http.Error(w, "Account suspended", http.StatusForbidden)
				return
			}

			// Store the user in the request context
			ctx := context.WithValue(r.Context(), UserClaimsKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Helper functions for admin and regular auth
func AdminOnly(jwtService jwt_internal.JwtService, blacklistCache BlacklistCache, secureCookies bool) func(http.Handler) http.Handler {
	return Auth(jwtService, blacklistCache, secureCookies, true)
}

func NeedAuth(jwtService jwt_internal.JwtService, blacklistCache BlacklistCache, secureCookies bool) func(http.Handler) http.Handler {
	return Auth(jwtService, blacklistCache, secureCookies, false)
}

// Function to retrieve the user from the context
func GetUserFromContext(r *http.Request) *domain.User {
	user, ok := r.Context().Value(UserClaimsKey).(*domain.User)
	if !ok {
		return nil // Or handle the case where no user is in the context
	}
	return user
}
