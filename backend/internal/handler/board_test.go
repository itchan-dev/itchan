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
)

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

	route := "/v1/boards"
	router := mux.NewRouter()
	router.HandleFunc(route, h.CreateBoard).Methods("POST")
	requestBody := []byte(`{"name": "Test Board", "short_name": "tb"}`)

	// Test case 1: successful request
	mockService := &MockBoardService{
		MockCreate: func(name, shortName string) error {
			return nil
		},
	}
	h.board = mockService
	req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status %d, but got %d", http.StatusCreated, rr.Code)
	}

	// Test case 2: invalid request body
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer([]byte(`{ivalid json::}`)))

	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}

	// Test case 3: service error
	mockService = &MockBoardService{
		MockCreate: func(name, shortName string) error {
			return errors.New("mock create error")
		},
	}
	h.board = mockService // Set the new mock service

	rr = httptest.NewRecorder() // Reset recorder
	req = httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, but got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestGetBoardHandler(t *testing.T) {
	h := &handler{}

	router := mux.NewRouter()
	router.HandleFunc("/v1/{board}", h.GetBoard).Methods("GET")

	// Test case 1: successful
	mockService := &MockBoardService{
		MockGet: func(shortName string, page int) (*domain.Board, error) {
			return &domain.Board{Name: "Test", ShortName: shortName}, nil
		},
	}
	h.board = mockService

	req := httptest.NewRequest("GET", "/v1/board_name?page=1", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
	}

	var board domain.Board
	if err := json.NewDecoder(rr.Body).Decode(&board); err != nil {
		t.Errorf("error decoding response body: %v", err)
	}
	if board.ShortName != "board_name" {
		t.Errorf("expected board ShortName 'test', but got '%s'", board.ShortName)
	}

	// Test case 2: service error
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

	// Test case 3: bad pagination param
	req = httptest.NewRequest("GET", "/v1/board_name?page=abc", nil)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}

	// Test case 4: default pagination param
	mockService = &MockBoardService{
		MockGet: func(shortName string, page int) (*domain.Board, error) {
			if page != default_page {
				return nil, errors.New("Mock")
			}
			return &domain.Board{}, nil
		},
	}
	h.board = mockService

	req = httptest.NewRequest("GET", "/v1/board_name", nil)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
	}
}

func TestDeleteBoardHandler(t *testing.T) {
	h := &handler{}

	router := mux.NewRouter()
	router.HandleFunc("/v1/{board}", h.DeleteBoard).Methods("DELETE")

	// Test case 1: successful
	mockService := &MockBoardService{
		MockDelete: func(shortName string) error {
			return nil
		},
	}
	h.board = mockService

	req := httptest.NewRequest("DELETE", "/v1/board_name", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
	}

	// Test case 2: service error
	mockService = &MockBoardService{
		MockDelete: func(shortName string) error {
			return errors.New("Mock")
		},
	}
	h.board = mockService

	rr = httptest.NewRecorder()
	req = httptest.NewRequest("DELETE", "/v1/board_name", nil)

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, but got %d", http.StatusInternalServerError, rr.Code)
	}
}
