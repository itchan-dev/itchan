package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
)

type mockBoardAccess struct {
	allowedDomains map[string][]string
}

func (m *mockBoardAccess) AllowedDomains(board string) []string {
	return m.allowedDomains[board]
}

func TestRestrictBoardAccess(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		boardAccess    *mockBoardAccess
		expectedStatus int
		nextCalled     bool
	}{
		{
			name: "missing board in URL vars",
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/board/", nil)
			},
			boardAccess:    &mockBoardAccess{allowedDomains: map[string][]string{}},
			expectedStatus: http.StatusOK,
			nextCalled:     true,
		},
		{
			name: "no restrictions for board",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/board/public", nil)
				req = mux.SetURLVars(req, map[string]string{"board": "public"})
				return req
			},
			boardAccess:    &mockBoardAccess{allowedDomains: map[string][]string{}},
			expectedStatus: http.StatusOK,
			nextCalled:     true,
		},
		{
			name: "unauthenticated user",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/board/restricted", nil)
				req = mux.SetURLVars(req, map[string]string{"board": "restricted"})
				return req
			},
			boardAccess: &mockBoardAccess{allowedDomains: map[string][]string{
				"restricted": {"example.com"},
			}},
			expectedStatus: http.StatusOK,
			nextCalled:     true,
		},
		{
			name: "admin user bypass",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/board/restricted", nil)
				req = mux.SetURLVars(req, map[string]string{"board": "restricted"})
				ctx := context.WithValue(req.Context(), userClaimsKey, &domain.User{
					Id:    1,
					Email: "admin@other.com",
					Admin: true,
				})
				return req.WithContext(ctx)
			},
			boardAccess: &mockBoardAccess{allowedDomains: map[string][]string{
				"restricted": {"example.com"},
			}},
			expectedStatus: http.StatusOK,
			nextCalled:     true,
		},
		{
			name: "allowed domain",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/board/restricted", nil)
				req = mux.SetURLVars(req, map[string]string{"board": "restricted"})
				ctx := context.WithValue(req.Context(), userClaimsKey, &domain.User{
					Id:    1,
					Email: "user@example.com",
				})
				return req.WithContext(ctx)
			},
			boardAccess: &mockBoardAccess{allowedDomains: map[string][]string{
				"restricted": {"example.com"},
			}},
			expectedStatus: http.StatusOK,
			nextCalled:     true,
		},
		{
			name: "empty domains",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/board/restricted", nil)
				req = mux.SetURLVars(req, map[string]string{"board": "restricted"})
				ctx := context.WithValue(req.Context(), userClaimsKey, &domain.User{
					Id:    1,
					Email: "user@example.com",
				})
				return req.WithContext(ctx)
			},
			boardAccess:    &mockBoardAccess{allowedDomains: map[string][]string{}},
			expectedStatus: http.StatusOK,
			nextCalled:     true,
		},
		{
			name: "disallowed domain",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/board/restricted", nil)
				req = mux.SetURLVars(req, map[string]string{"board": "restricted"})
				ctx := context.WithValue(req.Context(), userClaimsKey, &domain.User{
					Id:    1,
					Email: "user@unauthorized.com",
				})
				return req.WithContext(ctx)
			},
			boardAccess: &mockBoardAccess{allowedDomains: map[string][]string{
				"restricted": {"example.com"},
			}},
			expectedStatus: http.StatusForbidden,
			nextCalled:     false,
		},
		{
			name: "malformed email",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/board/restricted", nil)
				req = mux.SetURLVars(req, map[string]string{"board": "restricted"})
				ctx := context.WithValue(req.Context(), userClaimsKey, &domain.User{
					Id:    1,
					Email: "invalid-email",
				})
				return req.WithContext(ctx)
			},
			boardAccess: &mockBoardAccess{allowedDomains: map[string][]string{
				"restricted": {"example.com"},
			}},
			expectedStatus: http.StatusForbidden,
			nextCalled:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := RestrictBoardAccess(tt.boardAccess)(next)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, tt.setupRequest())

			assert.Equal(t, tt.expectedStatus, rr.Code)
			assert.Equal(t, tt.nextCalled, nextCalled, "Next handler call mismatch")
		})
	}
}
