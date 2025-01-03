package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt_internal "github.com/itchan-dev/itchan/backend/internal/utils/jwt"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuth(t *testing.T) {
	jwtService := jwt_internal.New("test_secret", time.Hour)
	admin := &domain.User{Id: 1, Email: "test@example.com", Admin: true}
	tokenAdmin, _ := jwtService.NewToken(admin)
	user := &domain.User{Id: 1, Email: "test@example.com", Admin: false}
	token, _ := jwtService.NewToken(user)

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
			handler := Auth(jwtService, tt.adminOnly)(func(w http.ResponseWriter, r *http.Request) {
				user := GetUserFromContext(r)
				require.NotNil(t, user, "Auth should always propagate user thru context")
				assert.Equal(t, tt.expectedUser, user)

				w.WriteHeader(http.StatusOK)
			})
			handler(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code, "handler returned wrong status code")
		})
	}
}

// TestGetUserFromContext tests the GetUserFromContext function.
func TestGetUserFromContext(t *testing.T) {
	user := &domain.User{Id: 1, Email: "test@example.com", Admin: true}
	req := httptest.NewRequest("GET", "http://example.com", nil)
	ctx := context.WithValue(req.Context(), userClaimsKey, user)
	req = req.WithContext(ctx)

	retrievedUser := GetUserFromContext(req)
	assert.Equal(t, user, retrievedUser)

	req = httptest.NewRequest("GET", "http://example.com", nil)
	retrievedUser = GetUserFromContext(req)

	assert.Nil(t, retrievedUser, "Expected user to be nil")
}
