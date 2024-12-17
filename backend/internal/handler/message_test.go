package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/domain"
)

type MockMessageService struct {
	MockCreate func(board string, author *domain.User, text string, attachments []domain.Attachment, thread_id int64) (int64, error)
	MockGet    func(id int64) (*domain.Message, error)
	MockDelete func(board string, id int64) error
}

func (m *MockMessageService) Create(board string, author *domain.User, text string, attachments []domain.Attachment, thread_id int64) (int64, error) {
	log.Println("XYI")
	if m.MockCreate != nil {
		return m.MockCreate(board, author, text, attachments, thread_id)
	}
	return 0, nil // Default behavior
}

func (m *MockMessageService) Get(id int64) (*domain.Message, error) {
	if m.MockGet != nil {
		return m.MockGet(id)
	}
	return nil, nil // Default behavior
}

func (m *MockMessageService) Delete(board string, id int64) error {
	if m.MockDelete != nil {
		return m.MockDelete(board, id)
	}
	return nil // Default behavior
}

func TestCreateMessageHandler(t *testing.T) {
	h := &handler{}

	route := "/b/1"
	router := mux.NewRouter()
	router.HandleFunc("/{board}/{thread}", h.CreateMessage).Methods("POST")
	requestBody := []byte(`{"text": "test text", "attachments": ["one", "two"]}`)

	// Test case 1: successful request
	mockService := &MockMessageService{
		MockCreate: func(board string, author *domain.User, text string, attachments []domain.Attachment, thread_id int64) (int64, error) {
			return 1, nil
		},
	}
	h.message = mockService

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

	// Test case 5: bad thread_id
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/b/abc", bytes.NewBuffer(requestBody))

	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}

	// Test case 6: service error
	mockService = &MockMessageService{
		MockCreate: func(board string, author *domain.User, text string, attachments []domain.Attachment, thread_id int64) (int64, error) {
			return 0, errors.New("Mock")
		},
	}
	h.message = mockService

	req = httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
	ctx = context.WithValue(req.Context(), "uid", int64(123))
	req = req.WithContext(ctx)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, but got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestGetMessageHandler(t *testing.T) {
	h := &handler{}

	route := "/b/123/321"
	router := mux.NewRouter()
	router.HandleFunc("/{board}/{thread}/{message}", h.GetMessage).Methods("GET")

	// Test case 1: successful
	mockService := &MockMessageService{
		MockGet: func(id int64) (*domain.Message, error) {
			return &domain.Message{Id: id}, nil
		},
	}
	h.message = mockService

	req := httptest.NewRequest("GET", route, nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
		return
	}

	var msg domain.Message
	if err := json.NewDecoder(rr.Body).Decode(&msg); err != nil {
		t.Errorf("error decoding response body: %v", err)
	}
	if msg.Id != 321 {
		t.Errorf("expected thread id '321', but got '%d'", msg.Id)
	}

	// Test case 2: bad message id
	req = httptest.NewRequest("GET", "/b/123/abc", nil)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
	}

	// Test case 3: service error
	mockService = &MockMessageService{
		MockGet: func(id int64) (*domain.Message, error) {
			return nil, errors.New("Mock error")
		},
	}
	h.message = mockService

	req = httptest.NewRequest("GET", route, nil)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, but got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestDeleteMessageHandler(t *testing.T) {
	h := &handler{}

	route := "/b/123/321"
	router := mux.NewRouter()
	router.HandleFunc("/{board}/{thread}/{message}", h.DeleteMessage).Methods("DELETE")

	// Test case 1: successful
	mockService := &MockMessageService{
		MockDelete: func(board string, id int64) error {
			return nil
		},
	}
	h.message = mockService

	req := httptest.NewRequest("DELETE", route, nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, but got %d", http.StatusOK, rr.Code)
		return
	}

	// Test case 2: bad message id
	req = httptest.NewRequest("DELETE", "/b/321/abc", nil)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, but got %d", http.StatusBadRequest, rr.Code)
		return
	}

	// Test case 3: service error
	mockService = &MockMessageService{
		MockDelete: func(board string, id int64) error {
			return errors.New("Mock error")
		},
	}
	h.message = mockService

	req = httptest.NewRequest("DELETE", route, nil)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, but got %d", http.StatusInternalServerError, rr.Code)
		return
	}
}
