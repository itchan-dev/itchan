package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt_internal "github.com/itchan-dev/itchan/backend/internal/utils/jwt"
	"github.com/itchan-dev/itchan/shared/domain"
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
				if user == nil {
					t.Fatal("Auth should always propagate user thru context")
				}
				if user == nil || user.Id != tt.expectedUser.Id || user.Email != tt.expectedUser.Email || user.Admin != tt.expectedUser.Admin {
					t.Errorf("Expected user: %+v, got: %+v", tt.expectedUser, user)
				}

				w.WriteHeader(http.StatusOK)
			})
			handler(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned status code %v, want %v", status, tt.expectedStatus)
			}

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
	if retrievedUser == nil || retrievedUser.Id != user.Id || retrievedUser.Email != user.Email || retrievedUser.Admin != user.Admin {
		t.Errorf("Expected user: %+v, got: %+v", user, retrievedUser)
	}

	req = httptest.NewRequest("GET", "http://example.com", nil)
	retrievedUser = GetUserFromContext(req)

	if retrievedUser != nil {
		t.Errorf("Expected user: nil, got: %+v", retrievedUser)
	}
}
