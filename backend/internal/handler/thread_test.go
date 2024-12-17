package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/domain"
)

type MockThreadService struct {
	MockCreate func(title string, board string, msg *domain.Message) error
	MockGet    func(id int64) (*domain.Thread, error)
	MockDelete func(board string, id int64) error
}

func (m *MockThreadService) Create(title string, board string, msg *domain.Message) error {
	if m.MockCreate != nil {
		return m.MockCreate(title, board, msg)
	}
	return nil // Default behavior
}

func (m *MockThreadService) Get(id int64) (*domain.Thread, error) {
	if m.MockGet != nil {
		return m.MockGet(id)
	}
	return nil, nil // Default behavior
}

func (m *MockThreadService) Delete(board string, id int64) error {
	if m.MockDelete != nil {
		return m.MockDelete(board, id)
	}
	return nil // Default behavior
}

func TestCreateThreadHandler(t *testing.T) {
	h := &handler{}

	route := "/b"
	router := mux.NewRouter()
	router.HandleFunc("/{board}", h.CreateThread).Methods("POST")
	requestBody := []byte(`{"title": "thread title", "text": "test text", "attachments": ["one", "two"]}`)

	// Test case 1: successful request
	mockService := &MockThreadService{
		MockCreate: func(title string, board string, msg *domain.Message) error {
			return nil
		},
	}
	h.thread = mockService
	req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
	ctx := context.WithValue(req.Context(), "uid", int64(123))
	req = req.WithContext(ctx)
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

	// Test case 3: no uid in context
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))

	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, but got %d", http.StatusUnauthorized, rr.Code)
	}

	// Test case 4: bad uid type in context
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
	ctx = context.WithValue(req.Context(), "uid", "abc")
	req = req.WithContext(ctx)

	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, but got %d", http.StatusInternalServerError, rr.Code)
	}

	// Test case 5: service error
	mockService = &MockThreadService{
		MockCreate: func(title string, board string, msg *domain.Message) error {
			return errors.New("Mock error")
		},
	}
	h.thread = mockService

	req = httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
	ctx = context.WithValue(req.Context(), "uid", int64(123))
	req = req.WithContext(ctx)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, but got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestGetThreadHandler(t *testing.T) {
	h := &handler{}

	route := "/b/123"
	router := mux.NewRouter()
	router.HandleFunc("/{board}/{thread}", h.GetThread).Methods("GET")

	// Test case 1: successful
	mockService := &MockThreadService{
		MockGet: func(id int64) (*domain.Thread, error) {
			return &domain.Thread{Messages: []*domain.Message{{Id: id}}}, nil
		},
	}
	h.thread = mockService

	req := httptest.NewRequest("GET", route, nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
		return
	}

	var thread domain.Thread
	if err := json.NewDecoder(rr.Body).Decode(&thread); err != nil {
		t.Errorf("error decoding response body: %v", err)
	}
	if thread.Messages[0].Id != 123 {
		t.Errorf("expected thread id '123', but got '%d'", thread.Messages[0].Id)
	}

	// Test case 2: bad thread id
	req = httptest.NewRequest("GET", "/b/abc", nil)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}

	// Test case 3: service error
	mockService = &MockThreadService{
		MockGet: func(id int64) (*domain.Thread, error) {
			return nil, errors.New("Mock")
		},
	}
	h.thread = mockService
	req = httptest.NewRequest("GET", route, nil)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, but got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestDeleteThreadHandler(t *testing.T) {
	h := &handler{}

	route := "/b/123"
	router := mux.NewRouter()
	router.HandleFunc("/{board}/{thread}", h.DeleteThread).Methods("DELETE")

	// Test case 1: successful
	mockService := &MockThreadService{
		MockDelete: func(board string, id int64) error {
			return nil
		},
	}
	h.thread = mockService

	req := httptest.NewRequest("DELETE", route, nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
		return
	}

	// Test case 2: bad thread id
	req = httptest.NewRequest("DELETE", "/b/abc", nil)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
		return
	}

	// Test case 3: service error
	mockService = &MockThreadService{
		MockDelete: func(board string, id int64) error {
			return errors.New("Mock")
		},
	}
	h.thread = mockService
	req = httptest.NewRequest("DELETE", route, nil)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, but got %d", http.StatusInternalServerError, rr.Code)
		return
	}
}
