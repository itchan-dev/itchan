package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockThreadService struct {
	MockCreate       func(creationData domain.ThreadCreationData) (domain.ThreadId, domain.MsgId, error)
	MockGet          func(board domain.BoardShortName, id domain.ThreadId) (domain.Thread, error)
	MockDelete       func(board domain.BoardShortName, id domain.ThreadId) error
	MockTogglePinned func(board domain.BoardShortName, id domain.ThreadId) (bool, error)
}

func (m *MockThreadService) Create(creationData domain.ThreadCreationData) (domain.ThreadId, domain.MsgId, error) {
	if m.MockCreate != nil {
		return m.MockCreate(creationData)
	}
	return 1, 1, nil
}

func (m *MockThreadService) Get(board domain.BoardShortName, id domain.ThreadId) (domain.Thread, error) {
	if m.MockGet != nil {
		return m.MockGet(board, id)
	}
	return domain.Thread{Messages: []*domain.Message{{MessageMetadata: domain.MessageMetadata{Id: domain.MsgId(id)}}}}, nil
}

func (m *MockThreadService) Delete(board domain.BoardShortName, id domain.ThreadId) error {
	if m.MockDelete != nil {
		return m.MockDelete(board, id)
	}
	return nil
}

func (m *MockThreadService) TogglePinned(board domain.BoardShortName, id domain.ThreadId) (bool, error) {
	if m.MockTogglePinned != nil {
		return m.MockTogglePinned(board, id)
	}
	return true, nil
}

func setupThreadTestHandler(threadService service.ThreadService) (*Handler, *mux.Router) {
	cfg := &config.Config{
		Public: config.Public{
			MaxAttachmentsPerMessage: 4,
			MaxAttachmentSizeBytes:   10 * 1024 * 1024,
			MaxTotalAttachmentSize:   20 * 1024 * 1024,
			AllowedImageMimeTypes:    []string{"image/jpeg", "image/png", "image/gif"},
			AllowedVideoMimeTypes:    []string{"video/mp4", "video/webm"},
		},
	}
	h := &Handler{
		thread: threadService,
		cfg:    cfg,
	}
	router := mux.NewRouter()
	router.HandleFunc("/{board}", h.CreateThread).Methods(http.MethodPost)
	router.HandleFunc("/{board}/{thread}", h.GetThread).Methods(http.MethodGet)
	router.HandleFunc("/{board}/{thread}", h.DeleteThread).Methods(http.MethodDelete)

	return h, router
}

func TestCreateThreadHandler(t *testing.T) {
	boardName := "b"
	route := "/" + boardName
	testUser := domain.User{Id: 1, Email: "test@test.com"}
	expectedThreadID := domain.ThreadId(42)
	expectedOpMessageID := domain.MsgId(1)

	t.Run("successful request", func(t *testing.T) {
		mockService := &MockThreadService{
			MockCreate: func(data domain.ThreadCreationData) (domain.ThreadId, domain.MsgId, error) {
				assert.Equal(t, "thread title", data.Title)
				assert.Equal(t, boardName, data.Board)
				assert.Equal(t, testUser, data.OpMessage.Author)
				assert.Equal(t, "test text", data.OpMessage.Text)
				assert.Nil(t, data.OpMessage.ReplyTo)
				return expectedThreadID, expectedOpMessageID, nil
			},
		}
		_, router := setupThreadTestHandler(mockService)

		body := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(body)
		writer.WriteField("json", `{"title": "thread title", "op_message": {"text": "test text"}}`)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, route, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = addUserToContext(req, &testUser)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, fmt.Sprintf("%d", expectedThreadID), rr.Body.String())
	})

	t.Run("successful request with replies", func(t *testing.T) {
		mockService := &MockThreadService{
			MockCreate: func(data domain.ThreadCreationData) (domain.ThreadId, domain.MsgId, error) {
				require.NotNil(t, data.OpMessage.ReplyTo)
				require.Len(t, *data.OpMessage.ReplyTo, 1)
				reply := (*data.OpMessage.ReplyTo)[0]
				assert.Equal(t, domain.MsgId(123), reply.To)
				assert.Equal(t, domain.ThreadId(1), reply.ToThreadId)
				return expectedThreadID, expectedOpMessageID, nil
			},
		}
		_, router := setupThreadTestHandler(mockService)

		body := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(body)
		writer.WriteField("json", `{"title": "thread title", "op_message": {"text": "test text", "reply_to": [{"To": 123, "ToThreadId": 1, "From": 0, "FromThreadId": 0, "CreatedAt": "2023-01-01T00:00:00Z"}]}}`)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, route, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = addUserToContext(req, &testUser)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("validation error", func(t *testing.T) {
		_, router := setupThreadTestHandler(&MockThreadService{})

		body := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(body)
		writer.WriteField("json", `{ivalid json::}`)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, route, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = addUserToContext(req, &testUser)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("unauthorized access", func(t *testing.T) {
		_, router := setupThreadTestHandler(&MockThreadService{})

		body := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(body)
		writer.WriteField("json", `{"title": "thread title", "op_message": {"text": "test text"}}`)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, route, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("mock service error")
		mockService := &MockThreadService{
			MockCreate: func(data domain.ThreadCreationData) (domain.ThreadId, domain.MsgId, error) {
				return -1, -1, mockErr
			},
		}
		_, router := setupThreadTestHandler(mockService)

		body := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(body)
		writer.WriteField("json", `{"title": "thread title", "op_message": {"text": "test text"}}`)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, route, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = addUserToContext(req, &testUser)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestGetThreadHandler(t *testing.T) {
	boardName := "b"
	threadID := int64(123)
	route := fmt.Sprintf("/%s/%d", boardName, threadID)
	expectedThread := domain.Thread{
		ThreadMetadata: domain.ThreadMetadata{Title: "Test Thread", Board: boardName},
		Messages: []*domain.Message{
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
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		var actualThread domain.Thread
		err := json.Unmarshal(bytes.TrimSpace(rr.Body.Bytes()), &actualThread)
		require.NoError(t, err)
		assert.Equal(t, expectedThread, actualThread)
	})

	t.Run("invalid thread id", func(t *testing.T) {
		_, router := setupThreadTestHandler(&MockThreadService{})
		badRoute := "/" + boardName + "/abc"
		req := createRequest(t, http.MethodGet, badRoute, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("thread not found")
		mockService := &MockThreadService{
			MockGet: func(board domain.BoardShortName, id domain.MsgId) (domain.Thread, error) {
				return domain.Thread{}, mockErr
			},
		}
		_, router := setupThreadTestHandler(mockService)
		req := createRequest(t, http.MethodGet, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
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
		assert.Empty(t, rr.Body.String())
	})

	t.Run("invalid thread id", func(t *testing.T) {
		_, router := setupThreadTestHandler(&MockThreadService{})
		badRoute := "/" + boardName + "/abc"
		req := createRequest(t, http.MethodDelete, badRoute, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("permission denied to delete")
		mockService := &MockThreadService{
			MockDelete: func(board domain.BoardShortName, id domain.MsgId) error {
				return mockErr
			},
		}
		_, router := setupThreadTestHandler(mockService)
		req := createRequest(t, http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
