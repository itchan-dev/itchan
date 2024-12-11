package handler

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/config"
)

// Manual mock for BoardService
type MockAuthService struct {
	MockSignup func(email, password string) (int64, error)
	MockLogin  func(email, password string) (string, error)
}

func (m *MockAuthService) Signup(email, password string) (int64, error) {
	if m.MockSignup != nil {
		return m.MockSignup(email, password)
	}
	return 0, nil // Default behavior
}

func (m *MockAuthService) Login(email, password string) (string, error) {
	if m.MockLogin != nil {
		return m.MockLogin(email, password)
	}
	return "", nil // Default behavior
}

func TestAuthLoginHandler(t *testing.T) {
	// necessary in login method
	cfg := config.Config{Public: config.Public{JwtTTL: 999999999999}}

	h := &handler{cfg: &cfg} // Create handler

	route := "/v1/auth/login"
	router := mux.NewRouter()
	router.HandleFunc(route, h.Login).Methods("POST")

	// Test case 1: successful request
	mockService := &MockAuthService{
		MockLogin: func(email, password string) (string, error) {
			return "test_cookie", nil
		},
	}
	h.auth = mockService

	requestBody := []byte(`{"email": "123@mail.ru", "password": "test"}`)
	req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 {
		t.Errorf("expected one cookie set")
	}
	cookie := cookies[0]
	if cookie.Name != "accessToken" || cookie.Value != "test_cookie" {
		t.Errorf("expected accessToken cookie to have value 'test_cookie'")
	}

	// Test case 2: bad request body
	requestBody = []byte(`{"email": "123@mail.ru", "password":::: "test"}`)
	req = httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}

	// Test case 3: missing credentials
	requestBody = []byte(`{"email": "123@mail.ru"}`)
	req = httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}

	requestBody = []byte(`{"email": "123@mail.ru", "password": "test"}`)
	// Test case 4: wrong password
	mockService = &MockAuthService{
		MockLogin: func(email, password string) (string, error) {
			return "", internal_errors.WrongPassword
		},
	}
	h.auth = mockService
	req = httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, but got %d", http.StatusUnauthorized, rr.Code)
	}

	// Test case 5: invalid email format
	mockService = &MockAuthService{
		MockLogin: func(email, password string) (string, error) {
			return "", &internal_errors.ValidationError{Message: "Mock"}
		},
	}
	h.auth = mockService
	req = httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}

	// Test case 6: internal error
	mockService = &MockAuthService{
		MockLogin: func(email, password string) (string, error) {
			return "", errors.New("Mock")
		},
	}
	h.auth = mockService
	req = httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, but got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestAuthLogoutHandler(t *testing.T) {
	h := &handler{} // Create handler

	route := "/v1/auth/logout"
	router := mux.NewRouter()
	router.HandleFunc(route, h.Logout).Methods("POST")

	// Test case 1: successful request
	req := httptest.NewRequest(http.MethodPost, route, nil)
	rr := httptest.NewRecorder()
	cookie := &http.Cookie{
		Path:     "/",
		Name:     "accessToken",
		Value:    "abc",
		MaxAge:   9999,
		HttpOnly: true,
	}
	http.SetCookie(rr, cookie)

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
	}
	// should be 2 cookies with same name, and second one should be 0 MaxAge
	// in browser that means we will just delete cookie
	cookies := rr.Result().Cookies()
	if len(cookies) != 2 {
		t.Errorf("expected 2 cookies set, get %d", len(cookies))
	}
	if cookies[0].Name != cookies[1].Name {
		t.Errorf("expected cookies with same name, get %s %s", cookies[0].Name, cookies[1].Name)
	}
	if cookies[1].MaxAge > 0 {
		t.Errorf("expected second cookie MaxAge > 0, get %d", cookies[1].MaxAge)
	}
}
