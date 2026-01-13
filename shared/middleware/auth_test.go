package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	jwt_internal "github.com/itchan-dev/itchan/shared/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuth(t *testing.T) {
	jwtService := jwt_internal.New("test_secret", time.Hour)
	admin := &domain.User{Id: 1, Email: "test@example.com", Admin: true}
	tokenAdmin, _ := jwtService.NewToken(*admin)
	user := &domain.User{Id: 1, Email: "test@example.com", Admin: false}
	token, _ := jwtService.NewToken(*user)

	tests := []struct {
		name           string
		adminOnly      bool
		cookie         *http.Cookie
		expectedStatus int
		expectedUser   *domain.User
	}{
		{
			name:           "Valid token - Admin",
			adminOnly:      true,
			cookie:         &http.Cookie{Name: "accessToken", Value: tokenAdmin},
			expectedStatus: http.StatusOK,
			expectedUser:   admin,
		},
		{
			name:           "Valid token - Non-admin",
			adminOnly:      false,
			cookie:         &http.Cookie{Name: "accessToken", Value: token},
			expectedStatus: http.StatusOK,
			expectedUser:   user,
		},
		{
			name:           "No token",
			adminOnly:      false,
			cookie:         nil,
			expectedStatus: http.StatusUnauthorized,
			expectedUser:   nil,
		},
		{
			name:           "Invalid token",
			adminOnly:      false,
			cookie:         &http.Cookie{Name: "accessToken", Value: "invalid_token"},
			expectedStatus: http.StatusUnauthorized,
			expectedUser:   nil,
		},
		{
			name:           "Non-admin accessing admin route",
			adminOnly:      true,
			cookie:         &http.Cookie{Name: "accessToken", Value: token},
			expectedStatus: http.StatusForbidden,
			expectedUser:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			rr := httptest.NewRecorder()
			authMw := NewAuth(jwtService, nil, false)
			var middleware func(http.Handler) http.Handler
			if tt.adminOnly {
				middleware = authMw.AdminOnly()
			} else {
				middleware = authMw.NeedAuth()
			}
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user := GetUserFromContext(r)
				require.NotNil(t, user, "Auth should always propagate user thru context")
				if tt.expectedUser != nil {
					assert.Equal(t, tt.expectedUser.Id, user.Id)
					assert.Equal(t, tt.expectedUser.Email, user.Email)
					assert.Equal(t, tt.expectedUser.Admin, user.Admin)
					// Don't compare CreatedAt as JWT encoding/decoding can change timezone
				} else {
					assert.Equal(t, tt.expectedUser, user)
				}

				w.WriteHeader(http.StatusOK)
			}))
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code, "handler returned wrong status code")
		})
	}
}

func TestGetUserFromContext(t *testing.T) {
	// Test context without user
	t.Run("no user in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		assert.Nil(t, GetUserFromContext(req))
	})

	// Test context with user
	t.Run("user in context", func(t *testing.T) {
		user := &domain.User{Id: 1, Email: "test@example.com", Admin: true}
		req := httptest.NewRequest("GET", "/", nil)
		ctx := context.WithValue(req.Context(), UserClaimsKey, user)
		req = req.WithContext(ctx)

		assert.Equal(t, user, GetUserFromContext(req))
	})
}

// Mock blacklist cache for testing
type mockBlacklistCache struct {
	blacklistedUsers map[domain.UserId]bool
}

func (m *mockBlacklistCache) IsBlacklisted(userId domain.UserId) bool {
	if m == nil || m.blacklistedUsers == nil {
		return false
	}
	return m.blacklistedUsers[userId]
}

func TestAuthWithBlacklist(t *testing.T) {
	jwtService := jwt_internal.New("test_secret", time.Hour)
	user := &domain.User{Id: 1, Email: "test@example.com", Admin: false}
	blacklistedUser := &domain.User{Id: 2, Email: "banned@example.com", Admin: false}

	token, _ := jwtService.NewToken(*user)
	blacklistedToken, _ := jwtService.NewToken(*blacklistedUser)

	tests := []struct {
		name           string
		cookie         *http.Cookie
		blacklist      *mockBlacklistCache
		expectedStatus int
		shouldSetUser  bool
	}{
		{
			name:   "Non-blacklisted user with valid token",
			cookie: &http.Cookie{Name: "accessToken", Value: token},
			blacklist: &mockBlacklistCache{
				blacklistedUsers: map[domain.UserId]bool{},
			},
			expectedStatus: http.StatusOK,
			shouldSetUser:  true,
		},
		{
			name:   "Blacklisted user with valid token",
			cookie: &http.Cookie{Name: "accessToken", Value: blacklistedToken},
			blacklist: &mockBlacklistCache{
				blacklistedUsers: map[domain.UserId]bool{2: true},
			},
			expectedStatus: http.StatusForbidden,
			shouldSetUser:  false,
		},
		{
			name:   "User with valid token and nil blacklist cache",
			cookie: &http.Cookie{Name: "accessToken", Value: token},
			blacklist: nil,
			expectedStatus: http.StatusOK,
			shouldSetUser:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			rr := httptest.NewRecorder()

			authMw := NewAuth(jwtService, tt.blacklist, false)
			handler := authMw.NeedAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user := GetUserFromContext(r)
				if tt.shouldSetUser {
					require.NotNil(t, user, "User should be set in context")
				}
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rr, req)
			assert.Equal(t, tt.expectedStatus, rr.Code, "handler returned wrong status code")

			// Check if cookie was cleared for blacklisted users
			if tt.expectedStatus == http.StatusForbidden {
				cookies := rr.Result().Cookies()
				var accessTokenCookie *http.Cookie
				for _, c := range cookies {
					if c.Name == "accessToken" {
						accessTokenCookie = c
						break
					}
				}
				require.NotNil(t, accessTokenCookie, "Access token cookie should be set")
				assert.Equal(t, "", accessTokenCookie.Value, "Cookie value should be cleared")
				assert.Equal(t, -1, accessTokenCookie.MaxAge, "Cookie should be expired")
			}
		})
	}
}
