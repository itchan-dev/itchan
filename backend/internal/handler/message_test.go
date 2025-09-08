package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/backend/internal/service" // Use service interface from internal/service
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware" // Use shared middleware
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockMessageService implements the service.MessageService interface
type MockMessageService struct {
	MockCreate func(creationData domain.MessageCreationData) (domain.MsgId, error)
	MockGet    func(board domain.BoardShortName, id domain.MsgId) (domain.Message, error)
	MockDelete func(board domain.BoardShortName, id domain.MsgId) error
}

func (m *MockMessageService) Create(creationData domain.MessageCreationData) (domain.MsgId, error) {
	if m.MockCreate != nil {
		return m.MockCreate(creationData)
	}
	return 0, nil // Default behavior
}

func (m *MockMessageService) Get(board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
	if m.MockGet != nil {
		return m.MockGet(board, id)
	}
	return domain.Message{}, nil // Default behavior
}

func (m *MockMessageService) Delete(board domain.BoardShortName, id domain.MsgId) error {
	if m.MockDelete != nil {
		return m.MockDelete(board, id)
	}
	return nil // Default behavior
}

// Setup function to create handler with mock service
func setupMessageTestHandler(messageService service.MessageService) (*Handler, *mux.Router) {
	h := &Handler{
		message: messageService,
		// auth, cfg, board, thread services would be added here if needed by message handlers
	}
	router := mux.NewRouter()
	// Define routes used in tests, matching refactored_package.txt
	router.HandleFunc("/{board}/{thread}", h.CreateMessage).Methods(http.MethodPost)
	router.HandleFunc("/{board}/{thread}/{message}", h.GetMessage).Methods(http.MethodGet)
	router.HandleFunc("/{board}/{thread}/{message}", h.DeleteMessage).Methods(http.MethodDelete)

	return h, router
}

func TestCreateMessageHandler(t *testing.T) {
	board := "b"
	threadId := domain.ThreadId(1)
	threadIdStr := strconv.FormatInt(threadId, 10)
	route := "/" + board + "/" + threadIdStr
	user := domain.User{Id: 1, Email: "test@test.com"}
	validRequestBody := []byte(`{"text": "test text", "attachments": ["one", "two"]}`)
	expectedText := "test text"
	expectedAttachments := &domain.Attachments{"one", "two"}

	// Test data for replies
	validRequestBodyWithReplies := []byte(`{"text": "test text", "attachments": ["one", "two"], "reply_to": [{"To": 123, "ToThreadId": 1, "From": 0, "FromThreadId": 0, "CreatedAt": "2023-01-01T00:00:00Z"}]}`)

	t.Run("successful request", func(t *testing.T) {
		expectedMsgId := domain.MsgId(123)
		mockService := &MockMessageService{
			MockCreate: func(data domain.MessageCreationData) (domain.MsgId, error) {
				assert.Equal(t, domain.BoardShortName(board), data.Board)
				assert.Equal(t, user, data.Author)
				assert.Equal(t, domain.MsgText(expectedText), data.Text)
				assert.Equal(t, expectedAttachments, data.Attachments)
				assert.Equal(t, threadId, data.ThreadId)
				return expectedMsgId, nil
			},
		}
		_, router := setupMessageTestHandler(mockService)

		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(validRequestBody))
		ctx := context.WithValue(req.Context(), mw.UserClaimsKey, &user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Empty(t, rr.Body.String(), "Expected empty body on successful creation")
	})

	t.Run("invalid request body json", func(t *testing.T) {
		mockService := &MockMessageService{} // Behavior doesn't matter
		_, router := setupMessageTestHandler(mockService)

		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer([]byte(`{invalid json::}`)))
		ctx := context.WithValue(req.Context(), mw.UserClaimsKey, &user) // Need user to get past auth check
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Body is invalid json") // Error from utils.DecodeValidate
	})

	t.Run("missing required field (text)", func(t *testing.T) {
		mockService := &MockMessageService{} // Behavior doesn't matter
		_, router := setupMessageTestHandler(mockService)
		invalidBody := []byte(`{"attachments": ["one"]}`) // Missing 'text'

		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(invalidBody))
		ctx := context.WithValue(req.Context(), mw.UserClaimsKey, &user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Required fields missing") // Error from utils.DecodeValidate (validator)
	})

	t.Run("no user in context", func(t *testing.T) {
		mockService := &MockMessageService{} // Behavior doesn't matter
		_, router := setupMessageTestHandler(mockService)

		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(validRequestBody))
		// No user injected into context
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "Unauthorized")
	})

	t.Run("bad threadId format", func(t *testing.T) {
		mockService := &MockMessageService{} // Behavior doesn't matter
		_, router := setupMessageTestHandler(mockService)
		badRoute := "/" + board + "/abc" // Non-integer threadId

		req := httptest.NewRequest(http.MethodPost, badRoute, bytes.NewBuffer(validRequestBody))
		ctx := context.WithValue(req.Context(), mw.UserClaimsKey, &user) // Need user to get past auth check
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Bad request") // Error from strconv.Atoi
	})

	t.Run("service error during creation", func(t *testing.T) {
		mockErr := errors.New("database insertion failed")
		mockService := &MockMessageService{
			MockCreate: func(data domain.MessageCreationData) (domain.MsgId, error) {
				// Basic check that data is passed before returning error
				assert.Equal(t, domain.BoardShortName(board), data.Board)
				return 0, mockErr
			},
		}
		_, router := setupMessageTestHandler(mockService)

		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(validRequestBody))
		ctx := context.WithValue(req.Context(), mw.UserClaimsKey, &user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		// Assuming utils.WriteErrorAndStatusCode maps generic errors to 500
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), mockErr.Error())
	})

	t.Run("successful request with replies", func(t *testing.T) {
		expectedMsgId := domain.MsgId(123)
		mockService := &MockMessageService{
			MockCreate: func(data domain.MessageCreationData) (domain.MsgId, error) {
				assert.Equal(t, domain.BoardShortName(board), data.Board)
				assert.Equal(t, user, data.Author)
				assert.Equal(t, domain.MsgText(expectedText), data.Text)
				assert.Equal(t, expectedAttachments, data.Attachments)
				assert.Equal(t, threadId, data.ThreadId)
				// Check that ReplyTo is not nil and has the expected structure
				require.NotNil(t, data.ReplyTo, "ReplyTo should not be nil")
				require.Len(t, *data.ReplyTo, 1, "Should have one reply")
				reply := (*data.ReplyTo)[0]
				assert.Equal(t, domain.MsgId(123), reply.To)
				assert.Equal(t, domain.ThreadId(1), reply.ToThreadId)
				return expectedMsgId, nil
			},
		}
		_, router := setupMessageTestHandler(mockService)

		req := httptest.NewRequest(http.MethodPost, route, bytes.NewBuffer(validRequestBodyWithReplies))
		ctx := context.WithValue(req.Context(), mw.UserClaimsKey, &user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Empty(t, rr.Body.String(), "Expected empty body on successful creation")
	})
}

func TestGetMessageHandler(t *testing.T) {
	board := "b"
	threadId := domain.ThreadId(123)
	msgId := domain.MsgId(321)
	threadIdStr := strconv.FormatInt(threadId, 10)
	msgIdStr := strconv.FormatInt(msgId, 10)
	route := "/" + board + "/" + threadIdStr + "/" + msgIdStr
	expectedMessage := domain.Message{
		MessageMetadata: domain.MessageMetadata{Id: msgId, ThreadId: threadId},
		Text:            "Existing message",
		Attachments:     nil,
		Replies:         domain.Replies{},
	}

	// Test message with replies
	expectedMessageWithReplies := domain.Message{
		MessageMetadata: domain.MessageMetadata{Id: msgId, ThreadId: threadId},
		Text:            "Message with replies",
		Attachments:     nil,
		Replies: domain.Replies{
			&domain.Reply{
				To:           456,
				ToThreadId:   1,
				From:         789,
				FromThreadId: 1,
				CreatedAt:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	t.Run("successful get", func(t *testing.T) {
		mockService := &MockMessageService{
			MockGet: func(board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
				assert.Equal(t, msgId, id)
				return expectedMessage, nil
			},
		}
		_, router := setupMessageTestHandler(mockService)

		req := httptest.NewRequest(http.MethodGet, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var actualMsg domain.Message
		err := json.Unmarshal(rr.Body.Bytes(), &actualMsg)
		require.NoError(t, err, "Failed to decode response body")
		assert.Equal(t, expectedMessage, actualMsg)
	})

	t.Run("bad message id format", func(t *testing.T) {
		mockService := &MockMessageService{} // Behavior doesn't matter
		_, router := setupMessageTestHandler(mockService)
		badRoute := "/" + board + "/" + threadIdStr + "/abc" // Non-integer messageId

		req := httptest.NewRequest(http.MethodGet, badRoute, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Bad request") // Error from strconv.Atoi
	})

	t.Run("service error during get", func(t *testing.T) {
		mockErr := errors.New("message not found in db")
		mockService := &MockMessageService{
			MockGet: func(board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
				assert.Equal(t, msgId, id)
				return domain.Message{}, mockErr
			},
		}
		_, router := setupMessageTestHandler(mockService)

		req := httptest.NewRequest(http.MethodGet, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		// Assuming utils.WriteErrorAndStatusCode maps generic errors to 500
		// (Could map specific errors like "not found" to 404 if implemented)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), mockErr.Error())
	})

	t.Run("successful get with replies", func(t *testing.T) {
		mockService := &MockMessageService{
			MockGet: func(board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
				assert.Equal(t, msgId, id)
				return expectedMessageWithReplies, nil
			},
		}
		_, router := setupMessageTestHandler(mockService)

		req := httptest.NewRequest(http.MethodGet, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var actualMsg domain.Message
		err := json.Unmarshal(rr.Body.Bytes(), &actualMsg)
		require.NoError(t, err, "Failed to decode response body")
		assert.Equal(t, expectedMessageWithReplies, actualMsg)
	})
}

func TestDeleteMessageHandler(t *testing.T) {
	board := "b"
	threadId := int64(123)
	msgId := int64(321)
	threadIdStr := strconv.FormatInt(threadId, 10)
	msgIdStr := strconv.FormatInt(msgId, 10)
	route := "/" + board + "/" + threadIdStr + "/" + msgIdStr

	t.Run("successful delete", func(t *testing.T) {
		mockService := &MockMessageService{
			MockDelete: func(b domain.BoardShortName, id domain.MsgId) error {
				assert.Equal(t, domain.BoardShortName(board), b)
				assert.Equal(t, msgId, id)
				return nil
			},
		}
		_, router := setupMessageTestHandler(mockService)

		req := httptest.NewRequest(http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Body.String(), "Expected empty body on successful deletion")
	})

	t.Run("bad message id format", func(t *testing.T) {
		mockService := &MockMessageService{} // Behavior doesn't matter
		_, router := setupMessageTestHandler(mockService)
		badRoute := "/" + board + "/" + threadIdStr + "/abc" // Non-integer messageId

		req := httptest.NewRequest(http.MethodDelete, badRoute, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Bad request") // Error from strconv.Atoi
	})

	t.Run("service error during delete", func(t *testing.T) {
		mockErr := errors.New("permission denied to delete")
		mockService := &MockMessageService{
			MockDelete: func(b domain.BoardShortName, id domain.MsgId) error {
				assert.Equal(t, domain.BoardShortName(board), b)
				assert.Equal(t, msgId, id)
				return mockErr
			},
		}
		_, router := setupMessageTestHandler(mockService)

		req := httptest.NewRequest(http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		// Assuming utils.WriteErrorAndStatusCode maps generic errors to 500
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), mockErr.Error())
	})
}
