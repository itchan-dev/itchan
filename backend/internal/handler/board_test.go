package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
)

type MockBoardService struct {
	MockCreate func(name, shortName string, allowedEmails *domain.Emails) error
	MockGet    func(shortName string, page int) (*domain.Board, error)
	MockDelete func(shortName string) error
}

func (m *MockBoardService) Create(name, shortName string, allowedEmails *domain.Emails) error {
	if m.MockCreate != nil {
		return m.MockCreate(name, shortName, allowedEmails)
	}
	return nil // Default behavior
}

func (m *MockBoardService) Get(shortName string, page int) (*domain.Board, error) {
	if m.MockGet != nil {
		return m.MockGet(shortName, page)
	}
	return nil, nil // Default behavior
}

func (m *MockBoardService) Delete(shortName string) error {
	if m.MockDelete != nil {
		return m.MockDelete(shortName)
	}
	return nil // Default behavior
}

func TestCreateBoardHandler(t *testing.T) {
	h := &Handler{} // Create handler

	route := "/v1/boards"
	router := mux.NewRouter()
	router.HandleFunc(route, h.CreateBoard).Methods("POST")
	requestBody := []byte(`{"name": "Test Board", "short_name": "tb"}`)

	t.Run("successful request", func(t *testing.T) {
		mockService := &MockBoardService{
			MockCreate: func(name, shortName string, allowedEmails *domain.Emails) error {
				return nil
			},
		}
		h.board = mockService
		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code, "expected status code %d, but got %d", http.StatusCreated, rr.Code)
	})

	t.Run("invalid request body", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer([]byte(`{ivalid json::}`)))

		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code, "expected status code %d, but got %d", http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &MockBoardService{
			MockCreate: func(name, shortName string, allowedEmails *domain.Emails) error {
				return errors.New("mock create error")
			},
		}
		h.board = mockService // Set the new mock service

		rr := httptest.NewRecorder() // Reset recorder
		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code, "expected status code %d, but got %d", http.StatusInternalServerError, rr.Code)
	})

	// Optional: Test case with AllowedEmails provided
	t.Run("with allowed emails", func(t *testing.T) {
		mockService := &MockBoardService{
			MockCreate: func(name, shortName string, allowedEmails *domain.Emails) error {
				if allowedEmails == nil || len(*allowedEmails) == 0 {
					return errors.New("allowedEmails expected")
				}
				return nil
			},
		}
		h.board = mockService

		requestBodyWithEmails := []byte(`{"name": "Test Board", "short_name": "tb", "allowed_emails": ["test@example.com"]}`)
		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBodyWithEmails))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})
}
func TestGetBoardHandler(t *testing.T) {
	h := &Handler{}

	router := mux.NewRouter()
	router.HandleFunc("/v1/{board}", h.GetBoard).Methods("GET")

	t.Run("successful", func(t *testing.T) {
		mockService := &MockBoardService{
			MockGet: func(shortName string, page int) (*domain.Board, error) {
				return &domain.Board{Name: "Test", ShortName: shortName}, nil
			},
		}
		h.board = mockService

		req := httptest.NewRequest("GET", "/v1/board_name?page=1", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "expected status code %d, but got %d", http.StatusOK, rr.Code)

		var board domain.Board
		err := json.NewDecoder(rr.Body).Decode(&board)
		assert.NoError(t, err, "error decoding response body")
		assert.Equal(t, "board_name", board.ShortName, "expected board ShortName 'board_name', but got '%s'", board.ShortName)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &MockBoardService{
			MockGet: func(shortName string, page int) (*domain.Board, error) {
				return nil, errors.New("Mock")
			},
		}
		h.board = mockService
		rr := httptest.NewRecorder()

		req := httptest.NewRequest("GET", "/v1/board_name?page=1", nil)
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code, "expected status code %d, but got %d", http.StatusInternalServerError, rr.Code)
	})

	t.Run("bad pagination param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/board_name?page=abc", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code, "expected status code %d, but got %d", http.StatusBadRequest, rr.Code)
	})

	t.Run("default pagination param", func(t *testing.T) {
		mockService := &MockBoardService{
			MockGet: func(shortName string, page int) (*domain.Board, error) {
				if page != default_page {
					return nil, errors.New("Mock")
				}
				return &domain.Board{}, nil
			},
		}
		h.board = mockService

		req := httptest.NewRequest("GET", "/v1/board_name", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "expected status code %d, but got %d", http.StatusOK, rr.Code)
	})
}

func TestDeleteBoardHandler(t *testing.T) {
	h := &Handler{}

	router := mux.NewRouter()
	router.HandleFunc("/v1/{board}", h.DeleteBoard).Methods("DELETE")

	t.Run("successful", func(t *testing.T) {
		mockService := &MockBoardService{
			MockDelete: func(shortName string) error {
				return nil
			},
		}
		h.board = mockService

		req := httptest.NewRequest("DELETE", "/v1/board_name", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "expected status code %d, but got %d", http.StatusOK, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &MockBoardService{
			MockDelete: func(shortName string) error {
				return errors.New("Mock")
			},
		}
		h.board = mockService

		rr := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", "/v1/board_name", nil)

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code, "expected status code %d, but got %d", http.StatusInternalServerError, rr.Code)
	})
}
