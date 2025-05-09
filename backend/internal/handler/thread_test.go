package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/backend/internal/service" // Use the service interface path
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockThreadService implements the service.ThreadService interface
type MockThreadService struct {
	MockCreate func(creationData domain.ThreadCreationData) (domain.MsgId, error)
	MockGet    func(board domain.BoardShortName, id domain.MsgId) (domain.Thread, error)
	MockDelete func(board domain.BoardShortName, id domain.MsgId) error
}

func (m *MockThreadService) Create(creationData domain.ThreadCreationData) (domain.MsgId, error) {
	if m.MockCreate != nil {
		return m.MockCreate(creationData)
	}
	return 1, nil // Default behavior
}

func (m *MockThreadService) Get(board domain.BoardShortName, id domain.MsgId) (domain.Thread, error) {
	if m.MockGet != nil {
		return m.MockGet(board, id)
	}
	// Return a default thread with at least one message so Id() doesn't panic
	return domain.Thread{Messages: []domain.Message{{MessageMetadata: domain.MessageMetadata{Id: id}}}}, nil // Default behavior
}

func (m *MockThreadService) Delete(board domain.BoardShortName, id domain.MsgId) error {
	if m.MockDelete != nil {
		return m.MockDelete(board, id)
	}
	return nil // Default behavior
}

// Setup function to create handler with mock service
func setupThreadTestHandler(threadService service.ThreadService) (*Handler, *mux.Router) {
	h := &Handler{
		thread: threadService,
		// auth, cfg, etc., could be added if needed by other parts of Handler
	}
	router := mux.NewRouter()
	// Define routes used in tests matching refactored_package.txt handlers
	router.HandleFunc("/{board}", h.CreateThread).Methods(http.MethodPost)
	router.HandleFunc("/{board}/{thread}", h.GetThread).Methods(http.MethodGet)
	router.HandleFunc("/{board}/{thread}", h.DeleteThread).Methods(http.MethodDelete)

	return h, router
}

func TestCreateThreadHandler(t *testing.T) {
	boardName := "b"
	route := "/" + boardName
	requestBody := []byte(`{"title": "thread title", "text": "test text", "attachments": ["one", "two"]}`)
	requestBodyNoAttach := []byte(`{"title": "thread title", "text": "test text"}`)
	testUser := domain.User{Id: 1, Email: "test@test.com"}
	expectedThreadID := domain.ThreadId(42)
	expectedTitle := "thread title"
	expectedText := "test text"
	expectedAttachments := &domain.Attachments{"one", "two"}

	t.Run("successful request with attachments", func(t *testing.T) {
		mockService := &MockThreadService{
			MockCreate: func(data domain.ThreadCreationData) (domain.MsgId, error) {
				assert.Equal(t, expectedTitle, data.Title)
				assert.Equal(t, boardName, data.Board)
				assert.Equal(t, testUser, data.OpMessage.Author)
				assert.Equal(t, expectedText, data.OpMessage.Text)
				require.NotNil(t, data.OpMessage.Attachments)
				assert.Equal(t, *expectedAttachments, *data.OpMessage.Attachments)
				return expectedThreadID, nil
			},
		}
		_, router := setupThreadTestHandler(mockService)

		req := createRequest(t, http.MethodPost, route, requestBody)
		ctx := context.WithValue(req.Context(), mw.UserClaimsKey, &testUser)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		// Check if the response body contains the created thread ID as a string
		assert.Equal(t, fmt.Sprintf("%d", expectedThreadID), rr.Body.String())
	})

	t.Run("successful request no attachments", func(t *testing.T) {
		mockService := &MockThreadService{
			MockCreate: func(data domain.ThreadCreationData) (domain.MsgId, error) {
				assert.Equal(t, expectedTitle, data.Title)
				assert.Equal(t, boardName, data.Board)
				assert.Equal(t, testUser, data.OpMessage.Author)
				assert.Equal(t, expectedText, data.OpMessage.Text)
				assert.Nil(t, data.OpMessage.Attachments, "Attachments should be nil when not provided")
				return expectedThreadID, nil
			},
		}
		_, router := setupThreadTestHandler(mockService)

		req := createRequest(t, http.MethodPost, route, requestBodyNoAttach)
		ctx := context.WithValue(req.Context(), mw.UserClaimsKey, &testUser)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, fmt.Sprintf("%d", expectedThreadID), rr.Body.String())
	})

	t.Run("invalid json request body", func(t *testing.T) {
		mockService := &MockThreadService{} // Behavior doesn't matter
		_, router := setupThreadTestHandler(mockService)
		req := createRequest(t, http.MethodPost, route, []byte(`{ivalid json::}`))
		// No need to inject user as it should fail before checking context
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Body is invalid json") // Based on utils.DecodeValidate error
	})

	t.Run("missing required field (title)", func(t *testing.T) {
		mockService := &MockThreadService{} // Behavior doesn't matter
		_, router := setupThreadTestHandler(mockService)
		// Missing "title" which is required by validate tag in handler
		invalidBody := []byte(`{"text": "test text only"}`)
		req := createRequest(t, http.MethodPost, route, invalidBody)
		// No need to inject user
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Required fields missing") // Based on utils.DecodeValidate error
	})

	t.Run("no user in context", func(t *testing.T) {
		mockService := &MockThreadService{} // Mock won't be called
		_, router := setupThreadTestHandler(mockService)
		req := createRequest(t, http.MethodPost, route, requestBody)
		// No user injected into context
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "Unauthorized")
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("mock service error")
		mockService := &MockThreadService{
			MockCreate: func(data domain.ThreadCreationData) (domain.MsgId, error) {
				// Can optionally assert arguments here too if needed
				return -1, mockErr
			},
		}
		_, router := setupThreadTestHandler(mockService)

		req := createRequest(t, http.MethodPost, route, requestBody)
		ctx := context.WithValue(req.Context(), mw.UserClaimsKey, &testUser)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code) // Assuming default mapping in WriteErrorAndStatusCode
		assert.Contains(t, rr.Body.String(), mockErr.Error())
	})
}

func TestGetThreadHandler(t *testing.T) {
	boardName := "b"
	threadID := int64(123)
	route := fmt.Sprintf("/%s/%d", boardName, threadID)
	expectedThread := domain.Thread{
		ThreadMetadata: domain.ThreadMetadata{Title: "Test Thread", Board: boardName},
		Messages: []domain.Message{
			{MessageMetadata: domain.MessageMetadata{Id: threadID, Author: domain.User{Id: 1, Email: "op@test.com"}}, Text: "OP message"},
			{MessageMetadata: domain.MessageMetadata{Id: 124, Author: domain.User{Id: 2, Email: "reply@test.com"}}, Text: "Reply message"},
		},
	}

	t.Run("successful get", func(t *testing.T) {
		mockService := &MockThreadService{
			MockGet: func(board domain.BoardShortName, id domain.MsgId) (domain.Thread, error) {
				assert.Equal(t, threadID, id)
				assert.Equal(t, boardName, board)
				return expectedThread, nil
			},
		}
		_, router := setupThreadTestHandler(mockService)

		req := createRequest(t, http.MethodGet, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "handler returned wrong content type")

		var actualThread domain.Thread
		// Use rr.Body directly as writeJSON adds a newline
		err := json.Unmarshal(bytes.TrimSpace(rr.Body.Bytes()), &actualThread)
		require.NoError(t, err, "error decoding response body")
		assert.Equal(t, expectedThread, actualThread)
	})

	t.Run("bad thread id (non-numeric)", func(t *testing.T) {
		mockService := &MockThreadService{} // Mock won't be called
		_, router := setupThreadTestHandler(mockService)
		badRoute := "/" + boardName + "/abc"
		req := createRequest(t, http.MethodGet, badRoute, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Bad request") // Message from handler
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("thread not found")
		mockService := &MockThreadService{
			MockGet: func(board domain.BoardShortName, id domain.MsgId) (domain.Thread, error) {
				assert.Equal(t, threadID, id)
				assert.Equal(t, boardName, board)
				return domain.Thread{}, mockErr
			},
		}
		_, router := setupThreadTestHandler(mockService)
		req := createRequest(t, http.MethodGet, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		// Assuming utils.WriteErrorAndStatusCode maps generic errors to 500
		// If a specific error (like a custom NotFoundError) mapped to 404, adjust this
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), mockErr.Error())
	})
}

func TestDeleteThreadHandler(t *testing.T) {
	boardName := "b"
	threadID := int64(123)
	route := fmt.Sprintf("/%s/%d", boardName, threadID)

	t.Run("successful deletion", func(t *testing.T) {
		mockService := &MockThreadService{
			MockDelete: func(board domain.BoardShortName, id domain.MsgId) error {
				assert.Equal(t, boardName, board)
				assert.Equal(t, threadID, id)
				return nil
			},
		}
		_, router := setupThreadTestHandler(mockService)

		req := createRequest(t, http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Body.String(), "Expected empty body on successful deletion")
	})

	t.Run("bad thread id (non-numeric)", func(t *testing.T) {
		mockService := &MockThreadService{} // Mock won't be called
		_, router := setupThreadTestHandler(mockService)
		badRoute := "/" + boardName + "/abc"
		req := createRequest(t, http.MethodDelete, badRoute, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Bad request") // Message from handler
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("permission denied to delete")
		mockService := &MockThreadService{
			MockDelete: func(board domain.BoardShortName, id domain.MsgId) error {
				assert.Equal(t, boardName, board)
				assert.Equal(t, threadID, id)
				return mockErr
			},
		}
		_, router := setupThreadTestHandler(mockService)
		req := createRequest(t, http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		// Assuming utils.WriteErrorAndStatusCode maps generic errors to 500
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), mockErr.Error())
	})
}
