package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCSRFMiddleware(t *testing.T) {
	// Test token generation
	t.Run("GenerateCSRFToken", func(t *testing.T) {
		handler := GenerateCSRFToken(CSRFConfig{SecureCookies: false})(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				token := GetCSRFTokenFromContext(r)
				if token == "" {
					t.Error("Expected CSRF token in context")
				}
				w.WriteHeader(http.StatusOK)
			}),
		)

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Check cookie was set
		cookies := w.Result().Cookies()
		found := false
		for _, cookie := range cookies {
			if cookie.Name == "csrf_token" && cookie.Value != "" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected CSRF cookie to be set")
		}
	})

	// Test token validation
	t.Run("ValidateCSRFToken", func(t *testing.T) {
		token := "test-token-123"

		tests := []struct {
			name           string
			method         string
			cookie         *http.Cookie
			formToken      string
			expectedStatus int
		}{
			{
				name:           "valid POST request",
				method:         "POST",
				cookie:         &http.Cookie{Name: "csrf_token", Value: token},
				formToken:      token,
				expectedStatus: http.StatusOK,
			},
			{
				name:           "GET request (no validation)",
				method:         "GET",
				cookie:         nil,
				formToken:      "",
				expectedStatus: http.StatusOK,
			},
			{
				name:           "missing cookie",
				method:         "POST",
				cookie:         nil,
				formToken:      token,
				expectedStatus: http.StatusForbidden,
			},
			{
				name:           "missing form token",
				method:         "POST",
				cookie:         &http.Cookie{Name: "csrf_token", Value: token},
				formToken:      "",
				expectedStatus: http.StatusForbidden,
			},
			{
				name:           "mismatched tokens",
				method:         "POST",
				cookie:         &http.Cookie{Name: "csrf_token", Value: token},
				formToken:      "different-token",
				expectedStatus: http.StatusForbidden,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				handler := ValidateCSRFToken()(
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
					}),
				)

				// Prepare form data
				form := url.Values{}
				if tt.formToken != "" {
					form.Set("csrf_token", tt.formToken)
				}

				req := httptest.NewRequest(tt.method, "/", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				if tt.cookie != nil {
					req.AddCookie(tt.cookie)
				}

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				if w.Code != tt.expectedStatus {
					t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
				}
			})
		}
	})
}
