package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/stretchr/testify/assert"
)

func setupInviteTestHandler(authService *MockAuthService) (*Handler, *chi.Mux) {
	h := &Handler{
		auth: authService,
	}
	router := chi.NewRouter()

	// Public routes
	router.Post("/v1/auth/register_with_invite", h.RegisterWithInvite)

	// Authenticated routes (with mock user in context)
	router.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Add mock user to context using the same key as auth middleware
				user := &domain.User{
					Id:        123,
					Admin:     false,
					CreatedAt: time.Now().Add(-30 * 24 * time.Hour), // 30 days old
				}
				ctx := context.WithValue(r.Context(), mw.UserClaimsKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		r.Post("/v1/invites", h.GenerateInvite)
		r.Get("/v1/invites", h.GetMyInvites)
		r.Delete("/v1/invites/{codeHash}", h.RevokeInvite)
	})

	return h, router
}

func TestRegisterWithInvite(t *testing.T) {
	route := "/v1/auth/register_with_invite"
	validRequestBody := []byte(`{"invite_code": "TESTCODE1234", "password": "password123"}`)
	expectedInviteCode := "TESTCODE1234"
	expectedPassword := domain.Password("password123")
	generatedEmail := "user123@invited.ru"

	t.Run("successful registration with invite", func(t *testing.T) {
		mockService := &MockAuthService{
			MockRegisterWithInvite: func(inviteCode string, password domain.Password) (string, error) {
				assert.Equal(t, expectedInviteCode, inviteCode)
				assert.Equal(t, expectedPassword, password)
				return generatedEmail, nil
			},
		}
		_, router := setupInviteTestHandler(mockService)

		req := createRequest(t, http.MethodPost, route, validRequestBody)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Registration successful")
		assert.Contains(t, rr.Body.String(), generatedEmail)
	})

	t.Run("validation error - missing invite code", func(t *testing.T) {
		_, router := setupInviteTestHandler(&MockAuthService{})
		req := createRequest(t, http.MethodPost, route, []byte(`{"password": "password123"}`))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("validation error - missing password", func(t *testing.T) {
		_, router := setupInviteTestHandler(&MockAuthService{})
		req := createRequest(t, http.MethodPost, route, []byte(`{"invite_code": "TESTCODE1234"}`))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("validation error - invalid json", func(t *testing.T) {
		_, router := setupInviteTestHandler(&MockAuthService{})
		req := createRequest(t, http.MethodPost, route, []byte(`{invalid`))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error - invalid invite code", func(t *testing.T) {
		mockErr := errors.New("invalid invite code")
		mockService := &MockAuthService{
			MockRegisterWithInvite: func(inviteCode string, password domain.Password) (string, error) {
				return "", mockErr
			},
		}
		_, router := setupInviteTestHandler(mockService)

		req := createRequest(t, http.MethodPost, route, validRequestBody)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestGenerateInvite(t *testing.T) {
	route := "/v1/invites"

	t.Run("successfully generate invite", func(t *testing.T) {
		expectedInvite := &domain.InviteCodeWithPlaintext{
			PlainCode: "NEWINVITECODE123",
			InviteCode: domain.InviteCode{
				CodeHash:  "hash123",
				CreatedBy: 123,
				CreatedAt: time.Now().UTC(),
				ExpiresAt: time.Now().UTC().Add(720 * time.Hour),
				UsedBy:    nil,
				UsedAt:    nil,
			},
		}

		mockService := &MockAuthService{
			MockGenerateInvite: func(user domain.User) (*domain.InviteCodeWithPlaintext, error) {
				assert.Equal(t, domain.UserId(123), user.Id)
				return expectedInvite, nil
			},
		}
		_, router := setupInviteTestHandler(mockService)

		req := createRequest(t, http.MethodPost, route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), expectedInvite.PlainCode)
		assert.Contains(t, rr.Body.String(), "expires_at")
	})

	t.Run("service error - invite limit reached", func(t *testing.T) {
		mockErr := errors.New("invite limit reached")
		mockService := &MockAuthService{
			MockGenerateInvite: func(user domain.User) (*domain.InviteCodeWithPlaintext, error) {
				return nil, mockErr
			},
		}
		_, router := setupInviteTestHandler(mockService)

		req := createRequest(t, http.MethodPost, route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestGetMyInvites(t *testing.T) {
	route := "/v1/invites"

	t.Run("successfully get user invites", func(t *testing.T) {
		now := time.Now().UTC()
		expectedInvites := []domain.InviteCode{
			{
				CodeHash:  "hash1",
				CreatedBy: 123,
				CreatedAt: now.Add(-2 * time.Hour),
				ExpiresAt: now.Add(24 * time.Hour),
				UsedBy:    nil,
				UsedAt:    nil,
			},
			{
				CodeHash:  "hash2",
				CreatedBy: 123,
				CreatedAt: now.Add(-1 * time.Hour),
				ExpiresAt: now.Add(48 * time.Hour),
				UsedBy:    nil,
				UsedAt:    nil,
			},
		}

		mockService := &MockAuthService{
			MockGetUserInvites: func(userId domain.UserId) ([]domain.InviteCode, error) {
				assert.Equal(t, domain.UserId(123), userId)
				return expectedInvites, nil
			},
		}
		_, router := setupInviteTestHandler(mockService)

		req := createRequest(t, http.MethodGet, route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "hash1")
		assert.Contains(t, rr.Body.String(), "hash2")
	})

	t.Run("return empty array when no invites", func(t *testing.T) {
		mockService := &MockAuthService{
			MockGetUserInvites: func(userId domain.UserId) ([]domain.InviteCode, error) {
				return nil, nil
			},
		}
		_, router := setupInviteTestHandler(mockService)

		req := createRequest(t, http.MethodGet, route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		// Should return empty array, not null
		assert.Equal(t, "[]\n", rr.Body.String())
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("database error")
		mockService := &MockAuthService{
			MockGetUserInvites: func(userId domain.UserId) ([]domain.InviteCode, error) {
				return nil, mockErr
			},
		}
		_, router := setupInviteTestHandler(mockService)

		req := createRequest(t, http.MethodGet, route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestRevokeInvite(t *testing.T) {
	codeHash := "hash123"
	route := "/v1/invites/" + codeHash

	t.Run("successfully revoke invite", func(t *testing.T) {
		mockService := &MockAuthService{
			MockRevokeInvite: func(userId domain.UserId, hash string) error {
				assert.Equal(t, domain.UserId(123), userId)
				assert.Equal(t, codeHash, hash)
				return nil
			},
		}
		_, router := setupInviteTestHandler(mockService)

		req := createRequest(t, http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invite revoked successfully")
	})

	t.Run("service error - invite not found", func(t *testing.T) {
		mockErr := errors.New("invite not found")
		mockService := &MockAuthService{
			MockRevokeInvite: func(userId domain.UserId, hash string) error {
				return mockErr
			},
		}
		_, router := setupInviteTestHandler(mockService)

		req := createRequest(t, http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("service error - invite already used", func(t *testing.T) {
		mockErr := errors.New("cannot revoke used invite")
		mockService := &MockAuthService{
			MockRevokeInvite: func(userId domain.UserId, hash string) error {
				return mockErr
			},
		}
		_, router := setupInviteTestHandler(mockService)

		req := createRequest(t, http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
