package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time" // Added for config

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/shared/config"

	"github.com/itchan-dev/itchan/shared/domain" // Import the domain package
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAuthService now implements the AuthService interface
type MockAuthService struct {
	MockRegister              func(creds domain.Credentials) error
	MockCheckConfirmationCode func(email domain.Email, confirmationCode string) error
	MockLogin                 func(creds domain.Credentials) (string, error)
}

func (m *MockAuthService) Register(creds domain.Credentials) error {
	if m.MockRegister != nil {
		return m.MockRegister(creds)
	}
	return nil // Default behavior
}

func (m *MockAuthService) CheckConfirmationCode(email domain.Email, confirmationCode string) error {
	if m.MockCheckConfirmationCode != nil {
		return m.MockCheckConfirmationCode(email, confirmationCode)
	}
	return nil // Default behavior
}

func (m *MockAuthService) Login(creds domain.Credentials) (string, error) {
	if m.MockLogin != nil {
		return m.MockLogin(creds)
	}
	return "", nil // Default behavior
}

// Setup function to create handler with mock service
func setupAuthTestHandler(authService service.AuthService, cfg *config.Config) (*Handler, *mux.Router) {
	if cfg == nil {
		// Provide a default config if none is given, especially for JwtTTL
		cfg = &config.Config{Public: config.Public{JwtTTL: 3600 * time.Second}}
	}
	h := &Handler{
		auth: authService,
		cfg:  cfg,
	}
	router := mux.NewRouter()
	// Define routes used in tests
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

	t.Run("invalid JSON", func(t *testing.T) {
		mockService := &MockAuthService{} // Behavior doesn't matter here
		_, router := setupAuthTestHandler(mockService, nil)
		req := createRequest(t, http.MethodPost, route, []byte(`{invalid`))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		// Optionally check error message if utils.WriteErrorAndStatusCode provides specific messages
	})

	t.Run("missing email", func(t *testing.T) {
		mockService := &MockAuthService{}
		_, router := setupAuthTestHandler(mockService, nil)
		invalidBody := []byte(`{"password": "password"}`)
		req := createRequest(t, http.MethodPost, route, invalidBody)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("missing password", func(t *testing.T) {
		mockService := &MockAuthService{}
		_, router := setupAuthTestHandler(mockService, nil)
		invalidBody := []byte(`{"email": "test@example.com"}`)
		req := createRequest(t, http.MethodPost, route, invalidBody)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("registration failed")
		mockService := &MockAuthService{
			MockRegister: func(creds domain.Credentials) error {
				assert.Equal(t, expectedEmail, creds.Email) // Still check input was passed correctly
				assert.Equal(t, expectedPassword, creds.Password)
				return mockErr
			},
		}
		_, router := setupAuthTestHandler(mockService, nil)

		req := createRequest(t, http.MethodPost, route, validRequestBody)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Assuming utils.WriteErrorAndStatusCode maps generic errors to 500
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		// Optionally check error message if utils.WriteErrorAndStatusCode provides specific messages
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
		assert.Empty(t, rr.Body.String()) // StatusOK with no body is typical for confirmation
	})

	t.Run("invalid JSON", func(t *testing.T) {
		mockService := &MockAuthService{}
		_, router := setupAuthTestHandler(mockService, nil)
		req := createRequest(t, http.MethodPost, route, []byte(`{invalid`))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("missing email", func(t *testing.T) {
		mockService := &MockAuthService{}
		_, router := setupAuthTestHandler(mockService, nil)
		invalidBody := []byte(`{"confirmation_code": "123456"}`)
		req := createRequest(t, http.MethodPost, route, invalidBody)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("missing confirmation code", func(t *testing.T) {
		mockService := &MockAuthService{}
		_, router := setupAuthTestHandler(mockService, nil)
		invalidBody := []byte(`{"email": "test@example.com"}`)
		req := createRequest(t, http.MethodPost, route, invalidBody)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("invalid code")
		mockService := &MockAuthService{
			MockCheckConfirmationCode: func(email domain.Email, code string) error {
				assert.Equal(t, expectedEmail, email)
				assert.Equal(t, expectedCode, code)
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
	// Use a non-zero TTL for realistic cookie MaxAge calculation
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
		require.Len(t, cookies, 1, "Expected exactly one cookie to be set")
		cookie := cookies[0]
		assert.Equal(t, "accessToken", cookie.Name)
		assert.Equal(t, expectedToken, cookie.Value)
		assert.True(t, cookie.HttpOnly, "Cookie should be HttpOnly")
		assert.Equal(t, "/", cookie.Path)
		// Check MaxAge corresponds roughly to TTL (allow for small differences)
		assert.InDelta(t, int(cfg.Public.JwtTTL.Seconds()), cookie.MaxAge, 1)
	})

	t.Run("invalid request body", func(t *testing.T) {
		mockService := &MockAuthService{}
		_, router := setupAuthTestHandler(mockService, cfg)
		req := createRequest(t, http.MethodPost, route, []byte(`{invalid json`))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("missing email", func(t *testing.T) {
		mockService := &MockAuthService{}
		_, router := setupAuthTestHandler(mockService, cfg)
		invalidBody := []byte(`{"password": "password"}`)
		req := createRequest(t, http.MethodPost, route, invalidBody)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("missing password", func(t *testing.T) {
		mockService := &MockAuthService{}
		_, router := setupAuthTestHandler(mockService, cfg)
		invalidBody := []byte(`{"email": "test@example.com"}`)
		req := createRequest(t, http.MethodPost, route, invalidBody)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("login failed")
		mockService := &MockAuthService{
			MockLogin: func(creds domain.Credentials) (string, error) {
				assert.Equal(t, expectedEmail, creds.Email)
				assert.Equal(t, expectedPassword, creds.Password)
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
	_, router := setupAuthTestHandler(nil, nil) // No service dependency for logout

	t.Run("successful logout", func(t *testing.T) {
		// Simulate an existing accessToken cookie
		existingCookie := &http.Cookie{
			Name:  "accessToken",
			Value: "some_valid_token",
			Path:  "/",
		}
		req := createRequest(t, http.MethodPost, route, nil, existingCookie)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Body.String()) // Logout usually has no response body

		cookies := rr.Result().Cookies()
		require.Len(t, cookies, 1, "Expected exactly one cookie to be set")
		clearedCookie := cookies[0]

		assert.Equal(t, "accessToken", clearedCookie.Name)
		assert.Equal(t, "", clearedCookie.Value, "Cookie value should be cleared")
		assert.Equal(t, -1, clearedCookie.MaxAge, "Cookie MaxAge should be -1 to expire immediately")
		assert.True(t, clearedCookie.HttpOnly, "Cookie should retain HttpOnly flag")
		assert.Equal(t, "/", clearedCookie.Path)
	})

	t.Run("logout without existing cookie", func(t *testing.T) {
		// Ensure it still works even if the cookie wasn't present
		req := createRequest(t, http.MethodPost, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		cookies := rr.Result().Cookies()
		require.Len(t, cookies, 1)
		clearedCookie := cookies[0]
		assert.Equal(t, "accessToken", clearedCookie.Name)
		assert.Equal(t, -1, clearedCookie.MaxAge)
	})
}
