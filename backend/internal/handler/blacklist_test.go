package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBlacklistTestHandler(authService service.AuthService) (*Handler, *mux.Router) {
	h := &Handler{
		auth: authService,
	}
	router := mux.NewRouter()
	router.HandleFunc("/v1/admin/users/{userId}/blacklist", h.BlacklistUser).Methods(http.MethodPost)
	router.HandleFunc("/v1/admin/users/{userId}/blacklist", h.UnblacklistUser).Methods(http.MethodDelete)
	router.HandleFunc("/v1/admin/blacklist/refresh", h.RefreshBlacklistCache).Methods(http.MethodPost)
	router.HandleFunc("/v1/admin/blacklist", h.GetBlacklistedUsers).Methods(http.MethodGet)

	return h, router
}

func TestBlacklistUser(t *testing.T) {
	adminUser := &domain.User{
		Id: 1,
	}
	targetUserId := domain.UserId(42)
	reason := "spam"

	t.Run("successful blacklist", func(t *testing.T) {
		mockAuth := &MockAuthService{
			MockBlacklistUser: func(userId domain.UserId, r string, blacklistedBy domain.UserId) error {
				assert.Equal(t, targetUserId, userId)
				assert.Equal(t, reason, r)
				assert.Equal(t, adminUser.Id, blacklistedBy)
				return nil
			},
		}
		_, router := setupBlacklistTestHandler(mockAuth)

		body := []byte(`{"reason": "spam"}`)
		req := createRequest(t, http.MethodPost, "/v1/admin/users/42/blacklist", body)
		req = addUserToContext(req, adminUser)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "User blacklisted successfully", rr.Body.String())
	})

	t.Run("invalid user ID", func(t *testing.T) {
		_, router := setupBlacklistTestHandler(&MockAuthService{})

		body := []byte(`{"reason": "spam"}`)
		req := createRequest(t, http.MethodPost, "/v1/admin/users/invalid/blacklist", body)
		req = addUserToContext(req, adminUser)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid user ID")
	})

	t.Run("invalid request body", func(t *testing.T) {
		_, router := setupBlacklistTestHandler(&MockAuthService{})

		body := []byte(`{invalid json`)
		req := createRequest(t, http.MethodPost, "/v1/admin/users/42/blacklist", body)
		req = addUserToContext(req, adminUser)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error on blacklist", func(t *testing.T) {
		mockErr := errors.New("database error")
		mockAuth := &MockAuthService{
			MockBlacklistUser: func(userId domain.UserId, r string, blacklistedBy domain.UserId) error {
				return mockErr
			},
		}
		_, router := setupBlacklistTestHandler(mockAuth)

		body := []byte(`{"reason": "spam"}`)
		req := createRequest(t, http.MethodPost, "/v1/admin/users/42/blacklist", body)
		req = addUserToContext(req, adminUser)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestUnblacklistUser(t *testing.T) {
	targetUserId := domain.UserId(42)

	t.Run("successful unblacklist", func(t *testing.T) {
		mockAuth := &MockAuthService{
			MockUnblacklistUser: func(userId domain.UserId) error {
				assert.Equal(t, targetUserId, userId)
				return nil
			},
		}
		_, router := setupBlacklistTestHandler(mockAuth)

		req := createRequest(t, http.MethodDelete, "/v1/admin/users/42/blacklist", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "User unblacklisted successfully", rr.Body.String())
	})

	t.Run("invalid user ID", func(t *testing.T) {
		_, router := setupBlacklistTestHandler(&MockAuthService{})

		req := createRequest(t, http.MethodDelete, "/v1/admin/users/invalid/blacklist", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid user ID")
	})

	t.Run("service error on unblacklist", func(t *testing.T) {
		mockErr := errors.New("database error")
		mockAuth := &MockAuthService{
			MockUnblacklistUser: func(userId domain.UserId) error {
				return mockErr
			},
		}
		_, router := setupBlacklistTestHandler(mockAuth)

		req := createRequest(t, http.MethodDelete, "/v1/admin/users/42/blacklist", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestRefreshBlacklistCache(t *testing.T) {
	t.Run("successful refresh", func(t *testing.T) {
		mockAuth := &MockAuthService{
			MockRefreshBlacklistCache: func() error {
				return nil
			},
		}
		_, router := setupBlacklistTestHandler(mockAuth)

		req := createRequest(t, http.MethodPost, "/v1/admin/blacklist/refresh", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Blacklist cache refreshed successfully", rr.Body.String())
	})

	t.Run("cache update failure", func(t *testing.T) {
		mockAuth := &MockAuthService{
			MockRefreshBlacklistCache: func() error {
				return errors.New("update failed")
			},
		}
		_, router := setupBlacklistTestHandler(mockAuth)

		req := createRequest(t, http.MethodPost, "/v1/admin/blacklist/refresh", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to refresh blacklist cache")
	})
}

func TestGetBlacklistedUsers(t *testing.T) {
	t.Run("successful retrieval with users", func(t *testing.T) {
		expectedEntries := []domain.BlacklistEntry{
			{
				UserId:        1,
				Reason:        "spam",
				BlacklistedAt: time.Now(),
				BlacklistedBy: 10,
			},
			{
				UserId:        2,
				Reason:        "harassment",
				BlacklistedAt: time.Now(),
				BlacklistedBy: 10,
			},
		}

		mockAuth := &MockAuthService{
			MockGetBlacklistedUsersWithDetails: func() ([]domain.BlacklistEntry, error) {
				return expectedEntries, nil
			},
		}
		_, router := setupBlacklistTestHandler(mockAuth)

		req := createRequest(t, http.MethodGet, "/v1/admin/blacklist", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response api.BlacklistResponse
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Len(t, response.Users, 2)
		assert.Equal(t, expectedEntries[0].UserId, response.Users[0].UserId)
		assert.Equal(t, expectedEntries[1].UserId, response.Users[1].UserId)
	})

	t.Run("successful retrieval with empty list", func(t *testing.T) {
		mockAuth := &MockAuthService{
			MockGetBlacklistedUsersWithDetails: func() ([]domain.BlacklistEntry, error) {
				return nil, nil
			},
		}
		_, router := setupBlacklistTestHandler(mockAuth)

		req := createRequest(t, http.MethodGet, "/v1/admin/blacklist", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response api.BlacklistResponse
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should return empty array, not null
		assert.NotNil(t, response.Users)
		assert.Len(t, response.Users, 0)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("database error")
		mockAuth := &MockAuthService{
			MockGetBlacklistedUsersWithDetails: func() ([]domain.BlacklistEntry, error) {
				return nil, mockErr
			},
		}
		_, router := setupBlacklistTestHandler(mockAuth)

		req := createRequest(t, http.MethodGet, "/v1/admin/blacklist", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
