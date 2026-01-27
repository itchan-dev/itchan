package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/itchan-dev/itchan/backend/internal/service"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockMessageService struct {
	MockCreate func(creationData domain.MessageCreationData) (domain.MsgId, error)
	MockGet    func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error)
	MockDelete func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) error
}

func (m *MockMessageService) Create(creationData domain.MessageCreationData) (domain.MsgId, error) {
	if m.MockCreate != nil {
		return m.MockCreate(creationData)
	}
	return 0, nil
}

func (m *MockMessageService) Get(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error) {
	if m.MockGet != nil {
		return m.MockGet(board, threadId, id)
	}
	return domain.Message{}, nil
}

func (m *MockMessageService) Delete(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) error {
	if m.MockDelete != nil {
		return m.MockDelete(board, threadId, id)
	}
	return nil
}

func setupMessageTestHandler(messageService service.MessageService) (*Handler, *chi.Mux) {
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
		message: messageService,
		cfg:     cfg,
	}
	router := chi.NewRouter()
	router.Post("/{board}/{thread}", h.CreateMessage)
	router.Get("/{board}/{thread}/{message}", h.GetMessage)
	router.Delete("/{board}/{thread}/{message}", h.DeleteMessage)

	return h, router
}

func TestCreateMessageHandler(t *testing.T) {
	board := "b"
	threadId := domain.ThreadId(1)
	threadIdStr := strconv.FormatInt(threadId, 10)
	route := "/" + board + "/" + threadIdStr
	user := domain.User{Id: 1}

	t.Run("successful request", func(t *testing.T) {
		expectedMsgId := domain.MsgId(123)
		mockService := &MockMessageService{
			MockCreate: func(data domain.MessageCreationData) (domain.MsgId, error) {
				assert.Equal(t, domain.BoardShortName(board), data.Board)
				assert.Equal(t, user, data.Author)
				assert.Equal(t, domain.MsgText("test text"), data.Text)
				assert.Equal(t, threadId, data.ThreadId)
				return expectedMsgId, nil
			},
		}
		_, router := setupMessageTestHandler(mockService)

		body := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(body)
		writer.WriteField("json", `{"text": "test text"}`)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, route, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = addUserToContext(req, &user)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(expectedMsgId), response["id"])
	})

	t.Run("successful request with replies", func(t *testing.T) {
		expectedMsgId := domain.MsgId(123)
		mockService := &MockMessageService{
			MockCreate: func(data domain.MessageCreationData) (domain.MsgId, error) {
				require.NotNil(t, data.ReplyTo)
				require.Len(t, *data.ReplyTo, 1)
				reply := (*data.ReplyTo)[0]
				assert.Equal(t, domain.MsgId(123), reply.To)
				assert.Equal(t, domain.ThreadId(1), reply.ToThreadId)
				return expectedMsgId, nil
			},
		}
		_, router := setupMessageTestHandler(mockService)

		body := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(body)
		writer.WriteField("json", `{"text": "test text", "reply_to": [{"To": 123, "ToThreadId": 1, "From": 0, "FromThreadId": 0, "CreatedAt": "2023-01-01T00:00:00Z"}]}`)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, route, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = addUserToContext(req, &user)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("validation error", func(t *testing.T) {
		_, router := setupMessageTestHandler(&MockMessageService{})

		body := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(body)
		writer.WriteField("json", `{invalid json::}`)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, route, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = addUserToContext(req, &user)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("unauthorized access", func(t *testing.T) {
		_, router := setupMessageTestHandler(&MockMessageService{})

		body := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(body)
		writer.WriteField("json", `{"text": "test text"}`)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, route, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("invalid threadId", func(t *testing.T) {
		_, router := setupMessageTestHandler(&MockMessageService{})
		badRoute := "/" + board + "/abc"

		body := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(body)
		writer.WriteField("json", `{"text": "test text"}`)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, badRoute, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = addUserToContext(req, &user)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("database insertion failed")
		mockService := &MockMessageService{
			MockCreate: func(data domain.MessageCreationData) (domain.MsgId, error) {
				return 0, mockErr
			},
		}
		_, router := setupMessageTestHandler(mockService)

		body := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(body)
		writer.WriteField("json", `{"text": "test text"}`)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, route, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = addUserToContext(req, &user)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
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
		MessageMetadata: domain.MessageMetadata{Id: msgId, ThreadId: threadId, Replies: domain.Replies{}},
		Text:            "Existing message",
		Attachments:     nil,
	}
	expectedMessageWithReplies := domain.Message{
		MessageMetadata: domain.MessageMetadata{Id: msgId, ThreadId: threadId, Replies: domain.Replies{
			&domain.Reply{
				To:           456,
				ToThreadId:   1,
				From:         789,
				FromThreadId: 1,
				CreatedAt:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}},
		Text:        "Message with replies",
		Attachments: nil,
	}

	t.Run("successful get", func(t *testing.T) {
		mockService := &MockMessageService{
			MockGet: func(board domain.BoardShortName, tid domain.ThreadId, id domain.MsgId) (domain.Message, error) {
				assert.Equal(t, threadId, tid)
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
		require.NoError(t, err)
		assert.Equal(t, expectedMessage, actualMsg)
	})

	t.Run("successful get with replies", func(t *testing.T) {
		mockService := &MockMessageService{
			MockGet: func(board domain.BoardShortName, tid domain.ThreadId, id domain.MsgId) (domain.Message, error) {
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
		require.NoError(t, err)
		assert.Equal(t, expectedMessageWithReplies, actualMsg)
	})

	t.Run("invalid message id", func(t *testing.T) {
		_, router := setupMessageTestHandler(&MockMessageService{})
		badRoute := "/" + board + "/" + threadIdStr + "/abc"

		req := httptest.NewRequest(http.MethodGet, badRoute, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("message not found in db")
		mockService := &MockMessageService{
			MockGet: func(board domain.BoardShortName, tid domain.ThreadId, id domain.MsgId) (domain.Message, error) {
				return domain.Message{}, mockErr
			},
		}
		_, router := setupMessageTestHandler(mockService)

		req := httptest.NewRequest(http.MethodGet, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
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
			MockDelete: func(b domain.BoardShortName, tid domain.ThreadId, id domain.MsgId) error {
				assert.Equal(t, domain.BoardShortName(board), b)
				assert.Equal(t, domain.ThreadId(threadId), tid)
				assert.Equal(t, domain.MsgId(msgId), id)
				return nil
			},
		}
		_, router := setupMessageTestHandler(mockService)

		req := httptest.NewRequest(http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Body.String())
	})

	t.Run("invalid message id", func(t *testing.T) {
		_, router := setupMessageTestHandler(&MockMessageService{})
		badRoute := "/" + board + "/" + threadIdStr + "/abc"

		req := httptest.NewRequest(http.MethodDelete, badRoute, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("service error", func(t *testing.T) {
		mockErr := errors.New("permission denied to delete")
		mockService := &MockMessageService{
			MockDelete: func(b domain.BoardShortName, tid domain.ThreadId, id domain.MsgId) error {
				return mockErr
			},
		}
		_, router := setupMessageTestHandler(mockService)

		req := httptest.NewRequest(http.MethodDelete, route, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
