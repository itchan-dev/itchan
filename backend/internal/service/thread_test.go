package service

import (
	"errors"
	"testing"

	internal_errors "github.com/itchan-dev/itchan/backend/internal/errors"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock ThreadStorage for testing
type MockThreadStorage struct {
	CreateThreadFunc func(title, board string, msg *domain.Message) (int64, error)
	GetThreadFunc    func(id int64) (*domain.Thread, error)
	DeleteThreadFunc func(board string, id int64) error
}

func (m *MockThreadStorage) CreateThread(title, board string, msg *domain.Message) (int64, error) {
	if m.CreateThreadFunc != nil {
		return m.CreateThreadFunc(title, board, msg)
	}
	return 1, nil
}

func (m *MockThreadStorage) GetThread(id int64) (*domain.Thread, error) {
	if m.GetThreadFunc != nil {
		return m.GetThreadFunc(id)
	}
	return &domain.Thread{Messages: []*domain.Message{&domain.Message{Id: id}}}, nil
}

func (m *MockThreadStorage) DeleteThread(board string, id int64) error {
	if m.DeleteThreadFunc != nil {
		return m.DeleteThreadFunc(board, id)
	}
	return nil
}

// Mock ThreadValidator for testing
type MockThreadValidator struct {
	TitleFunc func(title string) error
}

func (m *MockThreadValidator) Title(title string) error {
	if m.TitleFunc != nil {
		return m.TitleFunc(title)
	}
	return nil
}

func TestThreadCreate(t *testing.T) {
	storage := &MockThreadStorage{}
	validator := &MockThreadValidator{}
	service := NewThread(storage, validator)

	title := "test title"
	board := "test board"
	msg := &domain.Message{}

	t.Run("Successful creation", func(t *testing.T) {
		storage.CreateThreadFunc = func(tt, b string, m *domain.Message) (int64, error) {
			assert.Equal(t, title, tt)
			assert.Equal(t, board, b)
			return 1, nil
		}
		_, err := service.Create(title, board, msg)
		require.NoError(t, err)
	})

	t.Run("Validation error", func(t *testing.T) {
		validator.TitleFunc = func(t string) error {
			return &internal_errors.ErrorWithStatusCode{Message: "Invalid title", StatusCode: 400}
		}
		_, err := service.Create(title, board, msg)
		require.Error(t, err)
		assert.Equal(t, "Invalid title", err.Error())
	})
}

func TestThreadGet(t *testing.T) {
	storage := &MockThreadStorage{}
	validator := &MockThreadValidator{}
	service := NewThread(storage, validator)

	id := int64(1)

	t.Run("Successful get", func(t *testing.T) {
		expectedThread := &domain.Thread{Title: "test title", Messages: []*domain.Message{{Id: id}}}
		storage.GetThreadFunc = func(i int64) (*domain.Thread, error) {
			assert.Equal(t, id, i)
			return expectedThread, nil
		}
		thread, err := service.Get(id)
		require.NoError(t, err)
		assert.Equal(t, expectedThread.Id(), thread.Id())
		assert.Equal(t, expectedThread.Title, thread.Title)
	})

	t.Run("Storage error", func(t *testing.T) {
		mockError := errors.New("Mock GetThreadFunc")
		storage.GetThreadFunc = func(i int64) (*domain.Thread, error) { return nil, mockError }
		_, err := service.Get(id)
		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})
}

func TestThreadDelete(t *testing.T) {
	storage := &MockThreadStorage{}
	validator := &MockThreadValidator{}
	service := NewThread(storage, validator)

	board := "test board"
	id := int64(1)

	t.Run("Successful deletion", func(t *testing.T) {
		storage.DeleteThreadFunc = func(b string, i int64) error {
			assert.Equal(t, board, b)
			assert.Equal(t, id, i)
			return nil
		}
		err := service.Delete(board, id)
		require.NoError(t, err)
	})
}
