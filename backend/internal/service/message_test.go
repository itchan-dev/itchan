package service

import (
	"errors"
	"testing"

	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock structs
type MockMessageStorage struct {
	CreateMessageFunc func(board string, author *domain.User, text string, attachments *domain.Attachments, threadId int64) (int64, error)
	GetMessageFunc    func(id int64) (*domain.Message, error)
	DeleteMessageFunc func(board string, id int64) error
}

func (m *MockMessageStorage) CreateMessage(board string, author *domain.User, text string, attachments *domain.Attachments, threadId int64) (int64, error) {
	if m.CreateMessageFunc != nil {
		return m.CreateMessageFunc(board, author, text, attachments, threadId)
	}
	return 1, nil
}

func (m *MockMessageStorage) GetMessage(id int64) (*domain.Message, error) {
	if m.GetMessageFunc != nil {
		return m.GetMessageFunc(id)
	}
	return &domain.Message{Id: id}, nil
}

func (m *MockMessageStorage) DeleteMessage(board string, id int64) error {
	if m.DeleteMessageFunc != nil {
		return m.DeleteMessageFunc(board, id)
	}
	return nil
}

type MockMessageValidator struct {
	TextFunc func(text string) error
}

func (m *MockMessageValidator) Text(text string) error {
	if m.TextFunc != nil {
		return m.TextFunc(text)
	}
	return nil
}

func TestMessageCreate(t *testing.T) {
	storage := &MockMessageStorage{}
	validator := &MockMessageValidator{}
	service := NewMessage(storage, validator)

	board := "test_board"
	author := &domain.User{Id: 1}
	text := "test_text"
	attachments := domain.Attachments{}
	threadId := int64(1)

	t.Run("Successful creation", func(t *testing.T) {
		createdId, err := service.Create(board, author, text, &attachments, threadId)
		require.NoError(t, err)
		assert.Equal(t, int64(1), createdId)
	})

	t.Run("Storage error", func(t *testing.T) {
		mockError := errors.New("Mock CreateMessageFunc")
		storage.CreateMessageFunc = func(board string, author *domain.User, text string, attachments *domain.Attachments, threadId int64) (int64, error) {
			return 0, mockError
		}
		_, err := service.Create(board, author, text, &attachments, threadId)
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("Validation error", func(t *testing.T) {
		validator.TextFunc = func(text string) error {
			return &internal_errors.ErrorWithStatusCode{Message: "Invalid text", StatusCode: 400}
		}
		_, err := service.Create(board, author, text, &attachments, threadId)
		require.Error(t, err)
		assert.Equal(t, "Invalid text", err.Error())
	})
}

func TestMessageGet(t *testing.T) {
	storage := &MockMessageStorage{}
	validator := &MockMessageValidator{} // Not used in Get, but needed for constructor
	service := NewMessage(storage, validator)

	id := int64(1)

	t.Run("Successful get", func(t *testing.T) {
		expectedMessage := &domain.Message{Id: id, Text: "test_text"}
		storage.GetMessageFunc = func(i int64) (*domain.Message, error) {
			assert.Equal(t, id, i)
			return expectedMessage, nil
		}

		message, err := service.Get(id)
		require.NoError(t, err)
		assert.Equal(t, expectedMessage.Id, message.Id)
		assert.Equal(t, expectedMessage.Text, message.Text)
	})

	t.Run("Storage error", func(t *testing.T) {
		mockError := errors.New("Mock GetMessageFunc")
		storage.GetMessageFunc = func(id int64) (*domain.Message, error) { return nil, mockError }
		_, err := service.Get(id)
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})
}

func TestMessageDelete(t *testing.T) {
	storage := &MockMessageStorage{}
	validator := &MockMessageValidator{} // Not used in Delete, but needed for constructor
	service := NewMessage(storage, validator)

	board := "test_board"
	id := int64(1)

	t.Run("Successful delete", func(t *testing.T) {
		storage.DeleteMessageFunc = func(b string, i int64) error {
			assert.Equal(t, board, b)
			assert.Equal(t, id, i)
			return nil
		}
		err := service.Delete(board, id)
		require.NoError(t, err)
	})
}
