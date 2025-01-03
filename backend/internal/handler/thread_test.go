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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockThreadService struct {
	MockCreate func(title string, board string, msg *domain.Message) (int64, error)
	MockGet    func(id int64) (*domain.Thread, error)
	MockDelete func(board string, id int64) error
}

func (m *MockThreadService) Create(title string, board string, msg *domain.Message) (int64, error) {
	if m.MockCreate != nil {
		return m.MockCreate(title, board, msg)
	}
	return 1, nil // Default behavior
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
	h := &Handler{}
	route := "/b"
	router := mux.NewRouter()
	router.HandleFunc("/{board}", h.CreateThread).Methods("POST")
	requestBody := []byte(`{"title": "thread title", "text": "test text", "attachments": ["one", "two"]}`)

	t.Run("successful request", func(t *testing.T) {
		mockService := &MockThreadService{
			MockCreate: func(title string, board string, msg *domain.Message) (int64, error) {
				return 1, nil
			},
		}
		h.thread = mockService
		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
		ctx := context.WithValue(req.Context(), "uid", int64(123))
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("successful request no attachments", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer([]byte(`{"title": "thread title", "text": "test text"}`)))
		ctx := context.WithValue(req.Context(), "uid", int64(123))
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("invalid request body", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer([]byte(`{ivalid json::}`)))

		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("no uid in context", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))

		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("bad uid type in context", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
		ctx := context.WithValue(req.Context(), "uid", "abc")
		req = req.WithContext(ctx)

		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &MockThreadService{
			MockCreate: func(title string, board string, msg *domain.Message) (int64, error) {
				return -1, errors.New("Mock error")
			},
		}
		h.thread = mockService

		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
		ctx := context.WithValue(req.Context(), "uid", int64(123))
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestGetThreadHandler(t *testing.T) {
	h := &Handler{}
	route := "/b/123"
	router := mux.NewRouter()
	router.HandleFunc("/{board}/{thread}", h.GetThread).Methods("GET")

	t.Run("successful", func(t *testing.T) {
		mockService := &MockThreadService{
			MockGet: func(id int64) (*domain.Thread, error) {
				return &domain.Thread{Messages: []*domain.Message{{Id: id}}}, nil
			},
		}
		h.thread = mockService

		req := httptest.NewRequest("GET", route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var thread domain.Thread
		err := json.NewDecoder(rr.Body).Decode(&thread)
		require.NoError(t, err, "error decoding response body")
		assert.Equal(t, int64(123), thread.Messages[0].Id, "expected thread id '123'")
	})

	t.Run("bad thread id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/b/abc", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &MockThreadService{
			MockGet: func(id int64) (*domain.Thread, error) {
				return nil, errors.New("Mock")
			},
		}
		h.thread = mockService
		req := httptest.NewRequest("GET", route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestDeleteThreadHandler(t *testing.T) {
	h := &Handler{}
	route := "/b/123"
	router := mux.NewRouter()
	router.HandleFunc("/{board}/{thread}", h.DeleteThread).Methods("DELETE")

	t.Run("successful", func(t *testing.T) {
		mockService := &MockThreadService{
			MockDelete: func(board string, id int64) error {
				return nil
			},
		}
		h.thread = mockService

		req := httptest.NewRequest("DELETE", route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("bad thread id", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/b/abc", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &MockThreadService{
			MockDelete: func(board string, id int64) error {
				return errors.New("Mock")
			},
		}
		h.thread = mockService
		req := httptest.NewRequest("DELETE", route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
