package service

import (
	"database/sql"
	"errors"
	"sync" // Used for tracking calls in mocks safely in parallel tests
	"testing"

	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors" // Assuming this is the correct path based on broken_tests.txt
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

// MockMessageStorage mocks the MessageStorage interface.
type MockMessageStorage struct {
	createMessageFunc func(creationData domain.MessageCreationData, isOp bool, tx *sql.Tx) (domain.MsgId, error)
	getMessageFunc    func(board domain.BoardShortName, id domain.MsgId) (domain.Message, error)
	deleteMessageFunc func(board domain.BoardShortName, id domain.MsgId) error

	mu                    sync.Mutex
	createMessageCalled   bool
	createMessageArg      domain.MessageCreationData
	getMessageCalled      bool
	getMessageArgId       domain.MsgId
	deleteMessageCalled   bool
	deleteMessageArgBoard domain.BoardShortName
	deleteMessageArgId    domain.MsgId
}

func (m *MockMessageStorage) ResetCallTracking() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createMessageCalled = false
	m.createMessageArg = domain.MessageCreationData{}
	m.getMessageCalled = false
	m.getMessageArgId = 0
	m.deleteMessageCalled = false
	m.deleteMessageArgBoard = ""
	m.deleteMessageArgId = 0
}

func (m *MockMessageStorage) CreateMessage(creationData domain.MessageCreationData, isOp bool, tx *sql.Tx) (domain.MsgId, error) {
	m.mu.Lock()
	m.createMessageCalled = true
	m.createMessageArg = creationData
	m.mu.Unlock()

	if m.createMessageFunc != nil {
		return m.createMessageFunc(creationData, false, nil)
	}
	// Default success returns an arbitrary ID (e.g., 1)
	return 1, nil
}

func (m *MockMessageStorage) GetMessage(board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
	m.mu.Lock()
	m.getMessageCalled = true
	m.getMessageArgId = id
	m.mu.Unlock()

	if m.getMessageFunc != nil {
		return m.getMessageFunc(board, id)
	}
	// Default success returns a basic message matching the ID
	return domain.Message{MessageMetadata: domain.MessageMetadata{Id: id}}, nil
}

func (m *MockMessageStorage) DeleteMessage(board domain.BoardShortName, id domain.MsgId) error {
	m.mu.Lock()
	m.deleteMessageCalled = true
	m.deleteMessageArgBoard = board
	m.deleteMessageArgId = id
	m.mu.Unlock()

	if m.deleteMessageFunc != nil {
		return m.deleteMessageFunc(board, id)
	}
	return nil // Default success
}

// MockMessageValidator mocks the MessageValidator interface.
type MockMessageValidator struct {
	textFunc func(text domain.MsgText) error
}

func (m *MockMessageValidator) Text(text domain.MsgText) error {
	if m.textFunc != nil {
		return m.textFunc(text)
	}
	return nil // Default valid
}

// --- Tests ---

func TestMessageCreate(t *testing.T) {
	// Common test data
	testAuthor := domain.User{Id: 1}
	testAttachments := domain.Attachments{"file1.jpg"}
	testCreationData := domain.MessageCreationData{
		Board:       "tst",
		Author:      testAuthor,
		Text:        "Valid message text",
		Attachments: &testAttachments,
		ThreadId:    1,
	}
	expectedCreatedId := domain.MsgId(1)

	t.Run("Successful creation", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		storage.ResetCallTracking()
		validator := &MockMessageValidator{}
		service := NewMessage(storage, validator)

		validator.textFunc = func(text domain.MsgText) error {
			assert.Equal(t, testCreationData.Text, text)
			return nil // Validation passes
		}
		storage.createMessageFunc = func(creationData domain.MessageCreationData, isOp bool, tx *sql.Tx) (domain.MsgId, error) {
			assert.Equal(t, testCreationData, creationData)
			return expectedCreatedId, nil // Storage create succeeds
		}

		// Act
		createdId, err := service.Create(testCreationData)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedCreatedId, createdId)

		storage.mu.Lock() // Lock for reading tracker flags
		assert.True(t, storage.createMessageCalled, "Storage CreateMessage should be called")
		assert.False(t, storage.getMessageCalled, "GetMessage should not be called")
		assert.False(t, storage.deleteMessageCalled, "DeleteMessage should not be called")
		storage.mu.Unlock()
	})

	t.Run("Storage error during CreateMessage", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		storage.ResetCallTracking()
		validator := &MockMessageValidator{}
		service := NewMessage(storage, validator)
		storageError := errors.New("db write failed")

		validator.textFunc = func(text domain.MsgText) error {
			return nil // Validation passes
		}
		storage.createMessageFunc = func(creationData domain.MessageCreationData, isOp bool, tx *sql.Tx) (domain.MsgId, error) {
			assert.Equal(t, testCreationData, creationData)
			return 0, storageError // Storage create fails
		}

		// Act
		_, err := service.Create(testCreationData)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))

		storage.mu.Lock()
		assert.True(t, storage.createMessageCalled, "Storage CreateMessage should have been attempted")
		assert.False(t, storage.getMessageCalled, "GetMessage should not be called")
		assert.False(t, storage.deleteMessageCalled, "DeleteMessage should not be called")
		storage.mu.Unlock()
	})

	t.Run("Validation error", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		storage.ResetCallTracking()
		validator := &MockMessageValidator{}
		service := NewMessage(storage, validator)
		validationError := &internal_errors.ErrorWithStatusCode{Message: "Invalid text", StatusCode: 400}

		validator.textFunc = func(text domain.MsgText) error {
			assert.Equal(t, testCreationData.Text, text)
			return validationError // Validation fails
		}
		// storage.createMessageFunc is not set, so CreateMessage should not be called

		// Act
		_, err := service.Create(testCreationData)

		// Assert
		require.Error(t, err)
		assert.Equal(t, validationError, err) // Check for the specific validation error instance

		storage.mu.Lock()
		assert.False(t, storage.createMessageCalled, "CreateMessage should not be called on validation error")
		assert.False(t, storage.getMessageCalled, "GetMessage should not be called")
		assert.False(t, storage.deleteMessageCalled, "DeleteMessage should not be called")
		storage.mu.Unlock()
	})
}

func TestMessageGet(t *testing.T) {
	// Common test data
	testId := domain.MsgId(1)

	t.Run("Successful get", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		storage.ResetCallTracking()
		validator := &MockMessageValidator{} // Not used in Get, but needed for constructor
		service := NewMessage(storage, validator)
		expectedMessage := domain.Message{
			MessageMetadata: domain.MessageMetadata{Id: testId},
			Text:            "test_text",
		}

		storage.getMessageFunc = func(board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
			assert.Equal(t, testId, id)
			return expectedMessage, nil // Storage get succeeds
		}

		// Act
		message, err := service.Get("test", testId)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedMessage, message) // Compare the whole struct

		storage.mu.Lock()
		assert.True(t, storage.getMessageCalled, "Storage GetMessage should be called")
		assert.Equal(t, testId, storage.getMessageArgId)
		assert.False(t, storage.createMessageCalled, "CreateMessage should not be called")
		assert.False(t, storage.deleteMessageCalled, "DeleteMessage should not be called")
		storage.mu.Unlock()
	})

	t.Run("Storage error during GetMessage", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		storage.ResetCallTracking()
		validator := &MockMessageValidator{}
		service := NewMessage(storage, validator)
		storageError := errors.New("db read failed")

		storage.getMessageFunc = func(board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
			assert.Equal(t, testId, id)
			return domain.Message{}, storageError // Storage get fails
		}

		// Act
		_, err := service.Get("test", testId)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))

		storage.mu.Lock()
		assert.True(t, storage.getMessageCalled, "Storage GetMessage should have been attempted")
		assert.Equal(t, testId, storage.getMessageArgId)
		assert.False(t, storage.createMessageCalled, "CreateMessage should not be called")
		assert.False(t, storage.deleteMessageCalled, "DeleteMessage should not be called")
		storage.mu.Unlock()
	})
}

func TestMessageDelete(t *testing.T) {
	// Common test data
	testBoard := domain.BoardShortName("tst")
	testId := domain.MsgId(1)

	t.Run("Successful deletion", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		storage.ResetCallTracking()
		validator := &MockMessageValidator{} // Not used in Delete, but needed for constructor
		service := NewMessage(storage, validator)

		// Use built-in mock tracking via the func override
		storage.deleteMessageFunc = func(board domain.BoardShortName, id domain.MsgId) error {
			assert.Equal(t, testBoard, board)
			assert.Equal(t, testId, id)
			return nil // Storage delete succeeds
		}

		// Act
		err := service.Delete(testBoard, testId)

		// Assert
		require.NoError(t, err)

		storage.mu.Lock()
		assert.True(t, storage.deleteMessageCalled, "DeleteMessage should have been called")
		assert.Equal(t, testBoard, storage.deleteMessageArgBoard)
		assert.Equal(t, testId, storage.deleteMessageArgId)
		assert.False(t, storage.createMessageCalled, "CreateMessage should not be called")
		assert.False(t, storage.getMessageCalled, "GetMessage should not be called")
		storage.mu.Unlock()
	})

	t.Run("Storage error during DeleteMessage", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		storage.ResetCallTracking()
		validator := &MockMessageValidator{}
		service := NewMessage(storage, validator)
		storageError := errors.New("db delete failed")

		// Use built-in mock tracking via the func override
		storage.deleteMessageFunc = func(board domain.BoardShortName, id domain.MsgId) error {
			assert.Equal(t, testBoard, board)
			assert.Equal(t, testId, id)
			return storageError // Storage delete fails
		}

		// Act
		err := service.Delete(testBoard, testId)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))

		storage.mu.Lock()
		assert.True(t, storage.deleteMessageCalled, "DeleteMessage should have been attempted")
		assert.Equal(t, testBoard, storage.deleteMessageArgBoard) // Verify args even on failure
		assert.Equal(t, testId, storage.deleteMessageArgId)
		assert.False(t, storage.createMessageCalled, "CreateMessage should not be called")
		assert.False(t, storage.getMessageCalled, "GetMessage should not be called")
		storage.mu.Unlock()
	})
}
