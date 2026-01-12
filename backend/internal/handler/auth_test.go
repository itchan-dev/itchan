package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockAuthService struct {
	MockRegister                       func(creds domain.Credentials) error
	MockCheckConfirmationCode          func(email domain.Email, confirmationCode string) error
	MockLogin                          func(creds domain.Credentials) (string, error)
	MockBlacklistUser                  func(userId domain.UserId, reason string, blacklistedBy domain.UserId) error
	MockUnblacklistUser                func(userId domain.UserId) error
	MockGetBlacklistedUsersWithDetails func() ([]domain.BlacklistEntry, error)
	MockRefreshBlacklistCache          func() error
	MockRegisterWithInvite             func(inviteCode string, password domain.Password) (string, error)
	MockGenerateInvite                 func(user domain.User) (*domain.InviteCodeWithPlaintext, error)
	MockGetUserInvites                 func(userId domain.UserId) ([]domain.InviteCode, error)
	MockRevokeInvite                   func(userId domain.UserId, codeHash string) error
}

func (m *MockAuthService) Register(creds domain.Credentials) error {
	if m.MockRegister != nil {
		return m.MockRegister(creds)
	}
	return nil
}

func (m *MockAuthService) CheckConfirmationCode(email domain.Email, confirmationCode string) error {
	if m.MockCheckConfirmationCode != nil {
		return m.MockCheckConfirmationCode(email, confirmationCode)
	}
	return nil
}

func (m *MockAuthService) Login(creds domain.Credentials) (string, error) {
	if m.MockLogin != nil {
		return m.MockLogin(creds)
	}
	return "", nil
}

func (m *MockAuthService) BlacklistUser(userId domain.UserId, reason string, blacklistedBy domain.UserId) error {
	if m.MockBlacklistUser != nil {
		return m.MockBlacklistUser(userId, reason, blacklistedBy)
	}
	return nil
}

func (m *MockAuthService) UnblacklistUser(userId domain.UserId) error {
	if m.MockUnblacklistUser != nil {
		return m.MockUnblacklistUser(userId)
	}
	return nil
}

func (m *MockAuthService) GetBlacklistedUsersWithDetails() ([]domain.BlacklistEntry, error) {
	if m.MockGetBlacklistedUsersWithDetails != nil {
		return m.MockGetBlacklistedUsersWithDetails()
	}
	return nil, nil
}

func (m *MockAuthService) RefreshBlacklistCache() error {
	if m.MockRefreshBlacklistCache != nil {
		return m.MockRefreshBlacklistCache()
	}
	return nil
}

func (m *MockAuthService) RegisterWithInvite(inviteCode string, password domain.Password) (string, error) {
	if m.MockRegisterWithInvite != nil {
		return m.MockRegisterWithInvite(inviteCode, password)
	}
	return "generated@itchan.ru", nil
}

func (m *MockAuthService) GenerateInvite(user domain.User) (*domain.InviteCodeWithPlaintext, error) {
	if m.MockGenerateInvite != nil {
		return m.MockGenerateInvite(user)
	}
	return &domain.InviteCodeWithPlaintext{
		PlainCode: "test-invite-code",
		InviteCode: domain.InviteCode{
			CodeHash:  "hash",
			CreatedBy: user.Id,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
	}, nil
}

func (m *MockAuthService) GetUserInvites(userId domain.UserId) ([]domain.InviteCode, error) {
	if m.MockGetUserInvites != nil {
		return m.MockGetUserInvites(userId)
	}
	return []domain.InviteCode{}, nil
}

func (m *MockAuthService) RevokeInvite(userId domain.UserId, codeHash string) error {
	if m.MockRevokeInvite != nil {
		return m.MockRevokeInvite(userId, codeHash)
	}
	return nil
}

func setupAuthTestHandler(authService service.AuthService, cfg *config.Config) (*Handler, *mux.Router) {
	if cfg == nil {
		cfg = &config.Config{Public: config.Public{JwtTTL: 3600 * time.Second}}
	}
	h := &Handler{
		auth: authService,
		cfg:  cfg,
	}
	router := mux.NewRouter()
	router.HandleFunc("/v1/auth/register", h.Register).Methods(http.MethodPost)
	router.HandleFunc("/v1/auth/check-confirmation-code", h.CheckConfirmationCode).Methods(http.MethodPost)
	router.HandleFunc("/v1/auth/login", h.Login).Methods(http.MethodPost)
	router.HandleFunc("/v1/auth/logout", h.Logout).Methods(http.MethodPost)

	return h, router
}

func TestRegisterHandler(t *testing.T) {
	route := "/v1/auth/register"
	validRequestBody := []byte(`{"email": "test@example.com", "password": "password"}`)
	expectedEmail := domain.Email("test@example.com")
	expectedPassword := domain.Password("password")

	t.Run("successful registration", func(t *testing.T) {
		mockService := &MockAuthService{
			MockRegister: func(creds domain.Credentials) error {
				assert.Equal(t, expectedEmail, creds.Email)
				assert.Equal(t, expectedPassword, creds.Password)
				return nil
			},
		}
		_, router := setupAuthTestHandler(mockService, nil)

		req := createRequest(t, http.MethodPost, route, validRequestBody)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "The confirmation code has been sent by email", rr.Body.String())
	})

	t.Run("validation error", func(t *testing.T) {
		_, router := setupAuthTestHandler(&MockAuthService{}, nil)
		req := createRequest(t, http.MethodPost, route, []byte(`{invalid`))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("registration failed")
		mockService := &MockAuthService{
			MockRegister: func(creds domain.Credentials) error {
				return mockErr
			},
		}
		_, router := setupAuthTestHandler(mockService, nil)

		req := createRequest(t, http.MethodPost, route, validRequestBody)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestCheckConfirmationCodeHandler(t *testing.T) {
	route := "/v1/auth/check-confirmation-code"
	validRequestBody := []byte(`{"email": "test@example.com", "confirmation_code": "123456"}`)
	expectedEmail := domain.Email("test@example.com")
	expectedCode := "123456"

	t.Run("successful confirmation", func(t *testing.T) {
		mockService := &MockAuthService{
			MockCheckConfirmationCode: func(email domain.Email, code string) error {
				assert.Equal(t, expectedEmail, email)
				assert.Equal(t, expectedCode, code)
				return nil
			},
		}
		_, router := setupAuthTestHandler(mockService, nil)

		req := createRequest(t, http.MethodPost, route, validRequestBody)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Body.String())
	})

	t.Run("validation error", func(t *testing.T) {
		_, router := setupAuthTestHandler(&MockAuthService{}, nil)
		req := createRequest(t, http.MethodPost, route, []byte(`{"email": "test@example.com"}`))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("invalid code")
		mockService := &MockAuthService{
			MockCheckConfirmationCode: func(email domain.Email, code string) error {
				return mockErr
			},
		}
		_, router := setupAuthTestHandler(mockService, nil)

		req := createRequest(t, http.MethodPost, route, validRequestBody)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestAuthLoginHandler(t *testing.T) {
	cfg := &config.Config{Public: config.Public{JwtTTL: 3600 * time.Second}}
	route := "/v1/auth/login"
	requestBody := []byte(`{"email": "test@example.com", "password": "password"}`)
	expectedEmail := domain.Email("test@example.com")
	expectedPassword := domain.Password("password")
	expectedToken := "test_access_token"

	t.Run("successful login", func(t *testing.T) {
		mockService := &MockAuthService{
			MockLogin: func(creds domain.Credentials) (string, error) {
				assert.Equal(t, expectedEmail, creds.Email)
				assert.Equal(t, expectedPassword, creds.Password)
				return expectedToken, nil
			},
		}
		_, router := setupAuthTestHandler(mockService, cfg)

		req := createRequest(t, http.MethodPost, route, requestBody)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "You logged in", rr.Body.String())

		cookies := rr.Result().Cookies()
		require.Len(t, cookies, 1)
		cookie := cookies[0]
		assert.Equal(t, "accessToken", cookie.Name)
		assert.Equal(t, expectedToken, cookie.Value)
		assert.True(t, cookie.HttpOnly)
		assert.Equal(t, "/", cookie.Path)
		assert.InDelta(t, int(cfg.Public.JwtTTL.Seconds()), cookie.MaxAge, 1)
	})

	t.Run("validation error", func(t *testing.T) {
		_, router := setupAuthTestHandler(&MockAuthService{}, cfg)
		req := createRequest(t, http.MethodPost, route, []byte(`{"password": "password"}`))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("login failed")
		mockService := &MockAuthService{
			MockLogin: func(creds domain.Credentials) (string, error) {
				return "", mockErr
			},
		}
		_, router := setupAuthTestHandler(mockService, cfg)

		req := createRequest(t, http.MethodPost, route, requestBody)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestAuthLogoutHandler(t *testing.T) {
	route := "/v1/auth/logout"
	_, router := setupAuthTestHandler(nil, nil)

	t.Run("successful logout", func(t *testing.T) {
		existingCookie := &http.Cookie{
			Name:  "accessToken",
			Value: "some_valid_token",
			Path:  "/",
		}
		req := createRequest(t, http.MethodPost, route, nil, existingCookie)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Body.String())

		cookies := rr.Result().Cookies()
		require.Len(t, cookies, 1)
		clearedCookie := cookies[0]

		assert.Equal(t, "accessToken", clearedCookie.Name)
		assert.Equal(t, "", clearedCookie.Value)
		assert.Equal(t, -1, clearedCookie.MaxAge)
		assert.True(t, clearedCookie.HttpOnly)
		assert.Equal(t, "/", clearedCookie.Path)
	})

	t.Run("logout without existing cookie", func(t *testing.T) {
		req := createRequest(t, http.MethodPost, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		cookies := rr.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, -1, cookies[0].MaxAge)
	})
}
