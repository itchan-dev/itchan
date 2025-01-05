package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockAuthService struct {
	MockRegister              func(email, password string) (int64, error)
	MockCheckConfirmationCode func(email, confirmationCode string) error
	MockLogin                 func(email, password string) (string, error)
}

func (m *MockAuthService) Register(email, password string) (int64, error) {
	if m.MockRegister != nil {
		return m.MockRegister(email, password)
	}
	return 0, nil // Default behavior
}

func (m *MockAuthService) CheckConfirmationCode(email, confirmationCode string) error {
	if m.MockCheckConfirmationCode != nil {
		return m.MockCheckConfirmationCode(email, confirmationCode)
	}
	return nil // Default behavior
}

func (m *MockAuthService) Login(email, password string) (string, error) {
	if m.MockLogin != nil {
		return m.MockLogin(email, password)
	}
	return "", nil // Default behavior
}

func TestAuthLoginHandler(t *testing.T) {
	cfg := config.Config{Public: config.Public{JwtTTL: 999999999999}}
	h := &Handler{cfg: &cfg}

	route := "/v1/auth/login"
	router := mux.NewRouter()
	router.HandleFunc(route, h.Login).Methods("POST")
	requestBody := []byte(`{"email": "123@mail.ru", "password": "test"}`)

	t.Run("successful request", func(t *testing.T) {
		mockService := &MockAuthService{
			MockLogin: func(email, password string) (string, error) {
				return "test_cookie", nil
			},
		}
		h.auth = mockService

		req := createRequest(t, http.MethodPost, route, requestBody)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		cookies := rr.Result().Cookies()
		require.Len(t, cookies, 1)
		cookie := cookies[0]
		assert.Equal(t, "accessToken", cookie.Name)
		assert.Equal(t, "test_cookie", cookie.Value)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := createRequest(t, http.MethodPost, route, []byte(`{ivalid json::}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("Mock")
		mockService := &MockAuthService{
			MockLogin: func(email, password string) (string, error) {
				return "", mockErr
			},
		}
		h.auth = mockService

		req := createRequest(t, http.MethodPost, route, requestBody)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestAuthLogoutHandler(t *testing.T) {
	h := &Handler{}

	route := "/v1/auth/logout"
	router := mux.NewRouter()
	router.HandleFunc(route, h.Logout).Methods("POST")

	t.Run("successful request", func(t *testing.T) {
		cookie := &http.Cookie{
			Path:     "/",
			Name:     "accessToken",
			Value:    "abc",
			MaxAge:   9999,
			HttpOnly: true,
		}
		req := createRequest(t, http.MethodPost, route, nil, cookie)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		cookies := rr.Result().Cookies()
		require.Len(t, cookies, 1)

		assert.Equal(t, "accessToken", cookies[0].Name)
		assert.Less(t, cookies[0].MaxAge, 0)
	})
}
