package service

import (
	"errors"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock for UserActivityStorage ---

type MockUserActivityStorage struct {
	GetUserMessagesFunc func(userId domain.UserId, limit int) ([]domain.Message, error)
}

func (m *MockUserActivityStorage) GetUserMessages(userId domain.UserId, limit int) ([]domain.Message, error) {
	if m.GetUserMessagesFunc != nil {
		return m.GetUserMessagesFunc(userId, limit)
	}
	return []domain.Message{}, nil
}

// --- Tests ---

func TestGetUserActivity(t *testing.T) {
	userId := domain.UserId(42)
	messageLimit := 50

	t.Run("successfully get user activity", func(t *testing.T) {
		// Arrange
		now := time.Now().UTC()
		expectedMessages := []domain.Message{
			{
				MessageMetadata: domain.MessageMetadata{
					Board:     "tech",
					ThreadId:  10,
					Id:        1,
					Author:    domain.User{Id: userId},
					CreatedAt: now.Add(-1 * time.Hour),
				},
				Text: "Test message 1",
			},
			{
				MessageMetadata: domain.MessageMetadata{
					Board:     "dev",
					ThreadId:  11,
					Id:        2,
					Author:    domain.User{Id: userId},
					CreatedAt: now.Add(-2 * time.Hour),
				},
				Text: "Test message 2",
			},
		}

		storage := &MockUserActivityStorage{
			GetUserMessagesFunc: func(id domain.UserId, limit int) ([]domain.Message, error) {
				assert.Equal(t, userId, id)
				assert.Equal(t, messageLimit, limit)
				return expectedMessages, nil
			},
		}

		cfg := &config.Public{
			UserMessagesPageLimit: messageLimit,
		}

		service := NewUserActivity(storage, cfg)

		// Act
		response, err := service.GetUserActivity(userId)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, expectedMessages, response.Messages)
		assert.Len(t, response.Messages, 2)
	})

	t.Run("returns empty messages when user has no activity", func(t *testing.T) {
		// Arrange
		storage := &MockUserActivityStorage{
			GetUserMessagesFunc: func(id domain.UserId, limit int) ([]domain.Message, error) {
				return []domain.Message{}, nil
			},
		}

		cfg := &config.Public{
			UserMessagesPageLimit: messageLimit,
		}

		service := NewUserActivity(storage, cfg)

		// Act
		response, err := service.GetUserActivity(userId)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Empty(t, response.Messages)
	})

	t.Run("storage error", func(t *testing.T) {
		// Arrange
		mockError := errors.New("database connection error")
		storage := &MockUserActivityStorage{
			GetUserMessagesFunc: func(id domain.UserId, limit int) ([]domain.Message, error) {
				return nil, mockError
			},
		}

		cfg := &config.Public{
			UserMessagesPageLimit: messageLimit,
		}

		service := NewUserActivity(storage, cfg)

		// Act
		response, err := service.GetUserActivity(userId)

		// Assert
		require.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "failed to get user messages")
		assert.True(t, errors.Is(err, mockError))
	})

	t.Run("respects configured limit from config", func(t *testing.T) {
		// Arrange
		customLimit := 100
		var capturedLimit int

		storage := &MockUserActivityStorage{
			GetUserMessagesFunc: func(id domain.UserId, limit int) ([]domain.Message, error) {
				capturedLimit = limit
				return []domain.Message{}, nil
			},
		}

		cfg := &config.Public{
			UserMessagesPageLimit: customLimit,
		}

		service := NewUserActivity(storage, cfg)

		// Act
		_, err := service.GetUserActivity(userId)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, customLimit, capturedLimit, "Should use limit from config")
	})
}

func TestNewUserActivity(t *testing.T) {
	storage := &MockUserActivityStorage{}
	cfg := &config.Public{
		UserMessagesPageLimit: 50,
	}

	service := NewUserActivity(storage, cfg)

	assert.NotNil(t, service)

	// Verify service implements interface
	var _ UserActivityService = service
}
