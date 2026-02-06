package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/itchan-dev/itchan/shared/domain"
	jwt_internal "github.com/itchan-dev/itchan/shared/jwt"
	"github.com/itchan-dev/itchan/shared/logger"
	"github.com/itchan-dev/itchan/shared/utils"
)

// BlacklistCache interface defines methods needed by auth middleware
type BlacklistCache interface {
	IsBlacklisted(userId domain.UserId) bool
}

// Key to store the user claims in the request context
type key int

const UserClaimsKey key = 0

// Auth holds dependencies for authentication middleware
type Auth struct {
	jwtService     jwt_internal.JwtService
	blacklistCache BlacklistCache
	secureCookies  bool
}

// NewAuth creates a new Auth middleware instance
func NewAuth(jwtService jwt_internal.JwtService, blacklistCache BlacklistCache, secureCookies bool) *Auth {
	return &Auth{
		jwtService:     jwtService,
		blacklistCache: blacklistCache,
		secureCookies:  secureCookies,
	}
}

// NeedAuth returns middleware that requires authentication
func (a *Auth) NeedAuth() func(http.Handler) http.Handler {
	return a.auth(false)
}

// AdminOnly returns middleware that requires admin authentication
func (a *Auth) AdminOnly() func(http.Handler) http.Handler {
	return a.auth(true)
}

// OptionalAuth returns middleware that populates user context if token is valid, but doesn't require auth
func (a *Auth) OptionalAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, _ := a.extractUser(r)
			if user != nil {
				ctx := context.WithValue(r.Context(), UserClaimsKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// extractUser extracts and validates user from JWT token in request
// Returns (user, nil) on success, (nil, error) on failure
func (a *Auth) extractUser(r *http.Request) (*domain.User, error) {
	// Try to get token from cookie first (for browser clients)
	var tokenString string
	accessCookie, err := r.Cookie("accessToken")
	if err == nil {
		tokenString = accessCookie.Value
	} else if token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); found {
		// If no cookie, try Authorization header (for API/mobile clients)
		tokenString = token
	}

	if tokenString == "" {
		return nil, errNoToken
	}

	token, err := a.jwtService.DecodeToken(tokenString)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errInvalidClaims
	}

	uidFloat, ok := claims["uid"].(float64)
	if !ok {
		return nil, errInvalidClaims
	}

	emailDomain, ok := claims["email_domain"].(string)
	if !ok {
		return nil, errInvalidClaims
	}

	isAdmin, ok := claims["admin"].(bool)
	if !ok {
		return nil, errInvalidClaims
	}

	createdAtFloat, ok := claims["created_at"].(float64)
	if !ok {
		return nil, errInvalidClaims
	}

	user := &domain.User{
		Id:          int64(uidFloat),
		EmailDomain: emailDomain,
		Admin:       isAdmin,
		CreatedAt:   time.Unix(int64(createdAtFloat), 0),
	}

	if a.blacklistCache != nil && a.blacklistCache.IsBlacklisted(user.Id) {
		return nil, errBlacklisted
	}

	return user, nil
}

// Sentinel errors for extractUser
var (
	errNoToken       = errorString("no token")
	errInvalidClaims = errorString("invalid claims")
	errBlacklisted   = errorString("blacklisted")
)

type errorString string

func (e errorString) Error() string { return string(e) }

// auth is the internal method that implements the authentication logic
func (a *Auth) auth(adminOnly bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := a.extractUser(r)
			if err != nil {
				switch err {
				case errNoToken:
					http.Error(w, "Please sign-in", http.StatusUnauthorized)
				case errBlacklisted:
					// Clear JWT cookie to force re-login
					cookie := &http.Cookie{
						Path:     "/",
						Name:     "accessToken",
						Value:    "",
						MaxAge:   -1,
						HttpOnly: true,
						Secure:   a.secureCookies,
						SameSite: http.SameSiteLaxMode,
					}
					http.SetCookie(w, cookie)
					http.Error(w, "Account suspended", http.StatusForbidden)
				case errInvalidClaims:
					logger.Log.Error("invalid jwt claims")
					http.Error(w, "Invalid token", http.StatusUnauthorized)
				default:
					// Token decode error
					utils.WriteErrorAndStatusCode(w, err)
				}
				return
			}

			if adminOnly && !user.Admin {
				http.Error(w, "Access denied. Only for admin", http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), UserClaimsKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext retrieves the user from the context
func GetUserFromContext(r *http.Request) *domain.User {
	user, ok := r.Context().Value(UserClaimsKey).(*domain.User)
	if !ok {
		return nil // Or handle the case where no user is in the context
	}
	return user
}
