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
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockMessageService struct {
	MockCreate func(board string, author *domain.User, text string, attachments *domain.Attachments, threadId int64) (int64, error)
	MockGet    func(id int64) (*domain.Message, error)
	MockDelete func(board string, id int64) error
}

func (m *MockMessageService) Create(board string, author *domain.User, text string, attachments *domain.Attachments, threadId int64) (int64, error) {
	if m.MockCreate != nil {
		return m.MockCreate(board, author, text, attachments, threadId)
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
	h := &Handler{}
	route := "/b/1"
	router := mux.NewRouter()
	router.HandleFunc("/{board}/{thread}", h.CreateMessage).Methods("POST")
	requestBody := []byte(`{"text": "test text", "attachments": ["one", "two"]}`)
	user := domain.User{Id: 1, Email: "test@test.com"}

	t.Run("successful request", func(t *testing.T) {
		mockService := &MockMessageService{
			MockCreate: func(board string, author *domain.User, text string, attachments *domain.Attachments, threadId int64) (int64, error) {
				return 1, nil
			},
		}
		h.message = mockService

		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
		ctx := context.WithValue(req.Context(), mw.UserClaimsKey, &user)
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

	t.Run("no user in context", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))

		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("bad threadId", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/b/abc", bytes.NewBuffer(requestBody))

		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &MockMessageService{
			MockCreate: func(board string, author *domain.User, text string, attachments *domain.Attachments, threadId int64) (int64, error) {
				return 0, errors.New("Mock")
			},
		}
		h.message = mockService

		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(requestBody))
		ctx := context.WithValue(req.Context(), mw.UserClaimsKey, &user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestGetMessageHandler(t *testing.T) {
	h := &Handler{}
	route := "/b/123/321"
	router := mux.NewRouter()
	router.HandleFunc("/{board}/{thread}/{message}", h.GetMessage).Methods("GET")

	t.Run("successful", func(t *testing.T) {
		mockService := &MockMessageService{
			MockGet: func(id int64) (*domain.Message, error) {
				return &domain.Message{Id: id}, nil
			},
		}
		h.message = mockService

		req := httptest.NewRequest("GET", route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var msg domain.Message
		err := json.NewDecoder(rr.Body).Decode(&msg)
		require.NoError(t, err, "error decoding response body")
		assert.Equal(t, int64(321), msg.Id, "expected thread id '321'")
	})

	t.Run("bad message id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/b/123/abc", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &MockMessageService{
			MockGet: func(id int64) (*domain.Message, error) {
				return nil, errors.New("Mock error")
			},
		}
		h.message = mockService

		req := httptest.NewRequest("GET", route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestDeleteMessageHandler(t *testing.T) {
	h := &Handler{}
	route := "/b/123/321"
	router := mux.NewRouter()
	router.HandleFunc("/{board}/{thread}/{message}", h.DeleteMessage).Methods("DELETE")

	t.Run("successful", func(t *testing.T) {
		mockService := &MockMessageService{
			MockDelete: func(board string, id int64) error {
				return nil
			},
		}
		h.message = mockService

		req := httptest.NewRequest("DELETE", route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("bad message id", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/b/321/abc", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &MockMessageService{
			MockDelete: func(board string, id int64) error {
				return errors.New("Mock error")
			},
		}
		h.message = mockService

		req := httptest.NewRequest("DELETE", route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
