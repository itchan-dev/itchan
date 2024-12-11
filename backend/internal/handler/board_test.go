package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/domain"
)

// Manual mock for BoardService
type MockBoardService struct {
	MockCreate func(name, shortName string) error
	MockGet    func(shortName string, page int) (*domain.Board, error)
	MockDelete func(shortName string) error
}

func (m *MockBoardService) Create(name, shortName string) error {
	if m.MockCreate != nil {
		return m.MockCreate(name, shortName)
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
	h := &handler{} // Create handler

	// Test case 1: successful request
	mockService := &MockBoardService{
		MockCreate: func(name, shortName string) error {
			return nil
		},
	}
	h.board = mockService
	requestBody := []byte(`{"name": "Test Board", "short_name": "tb"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/boards", bytes.NewBuffer(requestBody))
	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/v1/boards", h.CreateBoard).Methods("POST")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status %d, but got %d", http.StatusCreated, rr.Code)
	}

	// Test case 2: invalid field in body
	requestBody = []byte(`{"name": "Test Board", "bad_key": "value"}`)
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/boards", bytes.NewBuffer(requestBody))

	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}

	// Test case 3: missing fields in body
	requestBody = []byte(`{"name": "Test Board"}`)
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/boards", bytes.NewBuffer(requestBody))

	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}

	// Test case 4: internal error in service.BoardService
	mockService = &MockBoardService{
		MockCreate: func(name, shortName string) error {
			return errors.New("mock create error")
		},
	}
	h.board = mockService // Set the new mock service
	requestBody = []byte(`{"name": "Test Board", "short_name": "tb"}`)
	rr = httptest.NewRecorder() // Reset recorder
	req = httptest.NewRequest(http.MethodPost, "/v1/boards", bytes.NewBuffer(requestBody))

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, but got %d", http.StatusInternalServerError, rr.Code)
	}

	// Test case 5: validation error in service.BoardService
	mockService = &MockBoardService{
		MockCreate: func(name, shortName string) error {
			return &internal_errors.ValidationError{Message: "Mock"}
		},
	}
	h.board = mockService       // Set the new mock service
	rr = httptest.NewRecorder() // Reset recorder
	req = httptest.NewRequest(http.MethodPost, "/v1/boards", bytes.NewBuffer(requestBody))

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGetBoardHandler(t *testing.T) {
	h := &handler{}

	// Test case 1: successful
	mockService := &MockBoardService{
		MockGet: func(shortName string, page int) (*domain.Board, error) {
			return &domain.Board{Name: "Test", ShortName: shortName}, nil
		},
	}
	h.board = mockService
	req := httptest.NewRequest("GET", "/v1/test", nil)

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/v1/{board}", h.GetBoard).Methods("GET")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
	}

	var board domain.Board
	if err := json.NewDecoder(rr.Body).Decode(&board); err != nil {
		t.Errorf("error decoding response body: %v", err)
	}
	if board.ShortName != "test" {
		t.Errorf("expected board ShortName 'test', but got '%s'", board.ShortName)
	}

	// Test case 2: not found
	mockService = &MockBoardService{
		MockGet: func(shortName string, page int) (*domain.Board, error) {
			return nil, internal_errors.NotFound
		},
	}
	h.board = mockService
	rr = httptest.NewRecorder()                        // Reset the recorder
	req = httptest.NewRequest("GET", "/v1/test2", nil) // New request for this scenario

	router.ServeHTTP(rr, req) // Use the *same* router

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, but got %d", http.StatusNotFound, rr.Code)
	}

	// Test case 3: validation error
	mockService = &MockBoardService{
		MockGet: func(shortName string, page int) (*domain.Board, error) {
			return nil, &internal_errors.ValidationError{Message: "Mock"}
		},
	}
	h.board = mockService
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}

	// Test case 4: internal error
	mockService = &MockBoardService{
		MockGet: func(shortName string, page int) (*domain.Board, error) {
			return nil, errors.New("Mock")
		},
	}
	h.board = mockService
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, but got %d", http.StatusInternalServerError, rr.Code)
	}

	// Test case 5: bad pagination param error
	req = httptest.NewRequest("GET", "/v1/test?page=abc", nil)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}

	// Test case 6: Validating pagination
	// At first successful run
	mockService = &MockBoardService{
		MockGet: func(shortName string, page int) (*domain.Board, error) {
			if page < 1 {
				return nil, errors.New("Mock")
			}
			return &domain.Board{Name: "Test", ShortName: shortName}, nil // Return a board
		},
	}
	h.board = mockService
	req = httptest.NewRequest("GET", "/v1/test3?page=2", nil)
	rr = httptest.NewRecorder() // Reset the recorder

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
	}
	if err := json.NewDecoder(rr.Body).Decode(&board); err != nil {
		t.Errorf("error decoding response body: %v", err)
	}
	if board.ShortName != "test3" {
		t.Errorf("expected board ShortName 'test3', but got '%s'", board.ShortName)
	}

	// Check that mock service raise error when page < 1
	_, err := mockService.MockGet("test", 0) // Pass invalid parameters and check response
	if err == nil {
		t.Error("MockGet with invalid parameters didn't return an error.")
	}

	// Check that handler fix page < 1
	req = httptest.NewRequest("GET", "/v1/test3?page=0", nil)
	rr = httptest.NewRecorder() // Reset the recorder

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
	}
	if err := json.NewDecoder(rr.Body).Decode(&board); err != nil {
		t.Errorf("error decoding response body: %v", err)
	}
	if board.ShortName != "test3" {
		t.Errorf("expected board ShortName 'test3', but got '%s'", board.ShortName)
	}

	// Check that default page > 0
	req = httptest.NewRequest("GET", "/v1/test3", nil)
	rr = httptest.NewRecorder() // Reset the recorder

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
	}
	if err := json.NewDecoder(rr.Body).Decode(&board); err != nil {
		t.Errorf("error decoding response body: %v", err)
	}
	if board.ShortName != "test3" {
		t.Errorf("expected board ShortName 'test3', but got '%s'", board.ShortName)
	}
}

func TestDeleteBoardHandler(t *testing.T) {
	h := &handler{}

	// Test case 1: successful
	mockService := &MockBoardService{
		MockDelete: func(shortName string) error {
			return nil
		},
	}
	h.board = mockService
	req := httptest.NewRequest("DELETE", "/v1/test", nil)
	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/v1/{board}", h.DeleteBoard).Methods("DELETE")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
	}

	// Test case 2: not found
	mockService = &MockBoardService{
		MockDelete: func(shortName string) error {
			return internal_errors.NotFound
		},
	}
	h.board = mockService
	rr = httptest.NewRecorder()                           // Reset the recorder
	req = httptest.NewRequest("DELETE", "/v1/test2", nil) // New request for this scenario

	router.ServeHTTP(rr, req) // Use the *same* router

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, but got %d", http.StatusNotFound, rr.Code)
	}

	// Test case 3: validation error
	mockService = &MockBoardService{
		MockDelete: func(shortName string) error {
			return &internal_errors.ValidationError{Message: "Mock"}
		},
	}
	h.board = mockService
	rr = httptest.NewRecorder()                           // Reset the recorder
	req = httptest.NewRequest("DELETE", "/v1/test2", nil) // New request for this scenario

	router.ServeHTTP(rr, req) // Use the *same* router

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}

	// Test case 4: internal error
	mockService = &MockBoardService{
		MockDelete: func(shortName string) error {
			return errors.New("Mock")
		},
	}
	h.board = mockService
	rr = httptest.NewRecorder()                           // Reset the recorder
	req = httptest.NewRequest("DELETE", "/v1/test2", nil) // New request for this scenario

	router.ServeHTTP(rr, req) // Use the *same* router

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, but got %d", http.StatusInternalServerError, rr.Code)
	}
}
