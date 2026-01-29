package service

import (
	"bytes"
	"errors"
	"sync" // Used for tracking calls in mocks safely in parallel tests
	"testing"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors" // Assuming this is the correct path based on broken_tests.txt
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helpers ---

func createDefaultTestConfig() *config.Public {
	return &config.Public{
		MaxAttachmentsPerMessage: 4,
		MaxAttachmentSizeBytes:   10 * 1024 * 1024,
		MaxTotalAttachmentSize:   20 * 1024 * 1024,
		AllowedImageMimeTypes:    []string{"image/jpeg", "image/png", "image/gif"},
		AllowedVideoMimeTypes:    []string{"video/mp4", "video/webm"},
	}
}

// --- Mocks ---

// MockMessageStorage mocks the MessageStorage interface.
type MockMessageStorage struct {
	createMessageFunc func(creationData domain.MessageCreationData, attachments domain.Attachments) (domain.MsgId, error)
	getMessageFunc    func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error)
	deleteMessageFunc func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) error

	mu                       sync.Mutex
	createMessageCalled      bool
	createMessageArg         domain.MessageCreationData
	createMessageAttachments domain.Attachments
	getMessageCalled         bool
	getMessageArgThreadId    domain.ThreadId
	getMessageArgId          domain.MsgId
	deleteMessageCalled      bool
	deleteMessageArgBoard    domain.BoardShortName
	deleteMessageArgThreadId domain.ThreadId
	deleteMessageArgId       domain.MsgId
}

func (m *MockMessageStorage) ResetCallTracking() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createMessageCalled = false
	m.createMessageArg = domain.MessageCreationData{}
	m.createMessageAttachments = nil
	m.getMessageCalled = false
	m.getMessageArgThreadId = 0
	m.getMessageArgId = 0
	m.deleteMessageCalled = false
	m.deleteMessageArgBoard = ""
	m.deleteMessageArgThreadId = 0
	m.deleteMessageArgId = 0
}

func (m *MockMessageStorage) CreateMessage(creationData domain.MessageCreationData, attachments domain.Attachments) (domain.MsgId, error) {
	m.mu.Lock()
	m.createMessageCalled = true
	m.createMessageArg = creationData
	m.createMessageAttachments = attachments
	m.mu.Unlock()

	if m.createMessageFunc != nil {
		return m.createMessageFunc(creationData, attachments)
	}
	// Default success returns an arbitrary ID (e.g., 1)
	return 1, nil
}

func (m *MockMessageStorage) GetMessage(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error) {
	m.mu.Lock()
	m.getMessageCalled = true
	m.getMessageArgThreadId = threadId
	m.getMessageArgId = id
	m.mu.Unlock()

	if m.getMessageFunc != nil {
		return m.getMessageFunc(board, threadId, id)
	}
	// Default success returns a basic message matching the ID
	return domain.Message{MessageMetadata: domain.MessageMetadata{Id: id, ThreadId: threadId}}, nil
}

func (m *MockMessageStorage) DeleteMessage(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) error {
	m.mu.Lock()
	m.deleteMessageCalled = true
	m.deleteMessageArgBoard = board
	m.deleteMessageArgThreadId = threadId
	m.deleteMessageArgId = id
	m.mu.Unlock()

	if m.deleteMessageFunc != nil {
		return m.deleteMessageFunc(board, threadId, id)
	}
	return nil // Default success
}

// MockMessageValidator mocks the MessageValidator interface.
type MockMessageValidator struct {
	textFunc         func(text domain.MsgText) error
	pendingFilesFunc func(files []*domain.PendingFile) error
}

func (m *MockMessageValidator) Text(text domain.MsgText) error {
	if m.textFunc != nil {
		return m.textFunc(text)
	}
	return nil // Default valid
}

func (m *MockMessageValidator) PendingFiles(files []*domain.PendingFile) error {
	if m.pendingFilesFunc != nil {
		return m.pendingFilesFunc(files)
	}
	return nil // Default valid
}

// --- Tests ---

func TestMessageCreate(t *testing.T) {
	// Common test data
	testAuthor := domain.User{Id: 1}
	testCreationData := domain.MessageCreationData{
		Board:    "tst",
		Author:   testAuthor,
		Text:     "Valid message text",
		ThreadId: 1,
	}
	expectedCreatedId := domain.MsgId(1)

	t.Run("Successful creation", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		storage.ResetCallTracking()
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())

		validator.textFunc = func(text domain.MsgText) error {
			assert.Equal(t, testCreationData.Text, text)
			return nil // Validation passes
		}
		storage.createMessageFunc = func(creationData domain.MessageCreationData, attachments domain.Attachments) (domain.MsgId, error) {
			assert.Equal(t, testCreationData, creationData)
			assert.Empty(t, attachments) // No attachments for text-only message
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
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())
		storageError := errors.New("db write failed")

		validator.textFunc = func(text domain.MsgText) error {
			return nil // Validation passes
		}
		storage.createMessageFunc = func(creationData domain.MessageCreationData, attachments domain.Attachments) (domain.MsgId, error) {
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
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())
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
	testThreadId := domain.ThreadId(1)
	testId := domain.MsgId(1)

	t.Run("Successful get", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		storage.ResetCallTracking()
		validator := &MockMessageValidator{} // Not used in Get, but needed for constructor
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())
		expectedMessage := domain.Message{
			MessageMetadata: domain.MessageMetadata{Id: testId, ThreadId: testThreadId},
			Text:            "test_text",
		}

		storage.getMessageFunc = func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error) {
			assert.Equal(t, testThreadId, threadId)
			assert.Equal(t, testId, id)
			return expectedMessage, nil // Storage get succeeds
		}

		// Act
		message, err := service.Get("test", testThreadId, testId)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedMessage, message) // Compare the whole struct

		storage.mu.Lock()
		assert.True(t, storage.getMessageCalled, "Storage GetMessage should be called")
		assert.Equal(t, testThreadId, storage.getMessageArgThreadId)
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
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())
		storageError := errors.New("db read failed")

		storage.getMessageFunc = func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error) {
			assert.Equal(t, testThreadId, threadId)
			assert.Equal(t, testId, id)
			return domain.Message{}, storageError // Storage get fails
		}

		// Act
		_, err := service.Get("test", testThreadId, testId)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))

		storage.mu.Lock()
		assert.True(t, storage.getMessageCalled, "Storage GetMessage should have been attempted")
		assert.Equal(t, testThreadId, storage.getMessageArgThreadId)
		assert.Equal(t, testId, storage.getMessageArgId)
		assert.False(t, storage.createMessageCalled, "CreateMessage should not be called")
		assert.False(t, storage.deleteMessageCalled, "DeleteMessage should not be called")
		storage.mu.Unlock()
	})
}

func TestMessageDelete(t *testing.T) {
	// Common test data
	testBoard := domain.BoardShortName("tst")
	testThreadId := domain.ThreadId(1)
	testId := domain.MsgId(1)

	t.Run("Successful deletion", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		storage.ResetCallTracking()
		validator := &MockMessageValidator{} // Not used in Delete, but needed for constructor
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())

		// Mock GetMessage to return a message with no attachments
		storage.getMessageFunc = func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error) {
			assert.Equal(t, testBoard, board)
			assert.Equal(t, testThreadId, threadId)
			assert.Equal(t, testId, id)
			return domain.Message{
				MessageMetadata: domain.MessageMetadata{Id: testId, ThreadId: testThreadId, Board: testBoard},
			}, nil
		}

		// Use built-in mock tracking via the func override
		storage.deleteMessageFunc = func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) error {
			assert.Equal(t, testBoard, board)
			assert.Equal(t, testThreadId, threadId)
			assert.Equal(t, testId, id)
			return nil // Storage delete succeeds
		}

		// Act
		err := service.Delete(testBoard, testThreadId, testId)

		// Assert
		require.NoError(t, err)

		storage.mu.Lock()
		assert.True(t, storage.deleteMessageCalled, "DeleteMessage should have been called")
		assert.Equal(t, testBoard, storage.deleteMessageArgBoard)
		assert.Equal(t, testThreadId, storage.deleteMessageArgThreadId)
		assert.Equal(t, testId, storage.deleteMessageArgId)
		assert.False(t, storage.createMessageCalled, "CreateMessage should not be called")
		assert.True(t, storage.getMessageCalled, "GetMessage should have been called")
		storage.mu.Unlock()
	})

	t.Run("Storage error during DeleteMessage", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		storage.ResetCallTracking()
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())
		storageError := errors.New("db delete failed")

		// Mock GetMessage to return a message with no attachments
		storage.getMessageFunc = func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error) {
			assert.Equal(t, testBoard, board)
			assert.Equal(t, testThreadId, threadId)
			assert.Equal(t, testId, id)
			return domain.Message{
				MessageMetadata: domain.MessageMetadata{Id: testId, ThreadId: testThreadId, Board: testBoard},
			}, nil
		}

		// Use built-in mock tracking via the func override
		storage.deleteMessageFunc = func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) error {
			assert.Equal(t, testBoard, board)
			assert.Equal(t, testThreadId, threadId)
			assert.Equal(t, testId, id)
			return storageError // Storage delete fails
		}

		// Act
		err := service.Delete(testBoard, testThreadId, testId)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))

		storage.mu.Lock()
		assert.True(t, storage.deleteMessageCalled, "DeleteMessage should have been attempted")
		assert.Equal(t, testBoard, storage.deleteMessageArgBoard) // Verify args even on failure
		assert.Equal(t, testThreadId, storage.deleteMessageArgThreadId)
		assert.Equal(t, testId, storage.deleteMessageArgId)
		assert.False(t, storage.createMessageCalled, "CreateMessage should not be called")
		assert.True(t, storage.getMessageCalled, "GetMessage should have been called")
		storage.mu.Unlock()
	})
}

func TestMessageCreate_TextOrAttachmentsRequired(t *testing.T) {
	t.Run("empty text and no files - should fail", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())

		testCreationData := domain.MessageCreationData{
			Board:        "tst",
			ThreadId:     1,
			Author:       domain.User{Id: 1},
			Text:         "",
			PendingFiles: nil,
		}

		// Act
		_, err := service.Create(testCreationData)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "message must contain either text or attachments")
	})

	t.Run("whitespace-only text and no files - should fail", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())

		testCreationData := domain.MessageCreationData{
			Board:        "tst",
			ThreadId:     1,
			Author:       domain.User{Id: 1},
			Text:         "   \t\n  ",
			PendingFiles: nil,
		}

		// Act
		_, err := service.Create(testCreationData)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "message must contain either text or attachments")
	})

	t.Run("valid text only - should succeed", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())

		testCreationData := domain.MessageCreationData{
			Board:        "tst",
			ThreadId:     1,
			Author:       domain.User{Id: 1},
			Text:         "Valid message text",
			PendingFiles: nil,
		}

		// Act
		msgId, err := service.Create(testCreationData)

		// Assert
		require.NoError(t, err)
		assert.NotZero(t, msgId)
	})

	t.Run("valid files only - should succeed", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())

		testCreationData := domain.MessageCreationData{
			Board:    "tst",
			ThreadId: 1,
			Author:   domain.User{Id: 1},
			Text:     "", // Empty text
			PendingFiles: []*domain.PendingFile{
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "test.jpg",
						SizeBytes: int64(len(loadTestImage(t))),
						MimeType:  "image/jpeg",
					},
					Data: bytes.NewReader(loadTestImage(t)),
				},
			},
		}

		// Act
		msgId, err := service.Create(testCreationData)

		// Assert
		require.NoError(t, err)
		assert.NotZero(t, msgId)
	})

	t.Run("both text and files - should succeed", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())

		testCreationData := domain.MessageCreationData{
			Board:    "tst",
			ThreadId: 1,
			Author:   domain.User{Id: 1},
			Text:     "Message with attachment",
			PendingFiles: []*domain.PendingFile{
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "test.jpg",
						SizeBytes: int64(len(loadTestImage(t))),
						MimeType:  "image/jpeg",
					},
					Data: bytes.NewReader(loadTestImage(t)),
				},
			},
		}

		// Act
		msgId, err := service.Create(testCreationData)

		// Assert
		require.NoError(t, err)
		assert.NotZero(t, msgId)
	})

	t.Run("invalid text and no files - should fail with text error", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())

		testCreationData := domain.MessageCreationData{
			Board:        "tst",
			ThreadId:     1,
			Author:       domain.User{Id: 1},
			Text:         "x", // Assume this is too short based on validator
			PendingFiles: nil,
		}

		// Mock validator to reject short text
		validator.textFunc = func(text domain.MsgText) error {
			return &internal_errors.ErrorWithStatusCode{
				Message:    "Text is too short",
				StatusCode: 400,
			}
		}

		// Act
		_, err := service.Create(testCreationData)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Text is too short")
	})

	t.Run("invalid files and no text - should fail with file error", func(t *testing.T) {
		// Arrange
		storage := &MockMessageStorage{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		service := NewMessage(storage, validator, mediaStorage, createDefaultTestConfig())

		testCreationData := domain.MessageCreationData{
			Board:    "tst",
			ThreadId: 1,
			Author:   domain.User{Id: 1},
			Text:     "",
			PendingFiles: []*domain.PendingFile{
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "test.exe",
						SizeBytes: 9,
						MimeType:  "application/x-executable", // Invalid MIME type
					},
					Data: bytes.NewReader([]byte("fake data")),
				},
			},
		}

		// Mock validator to reject invalid file
		validator.pendingFilesFunc = func(files []*domain.PendingFile) error {
			return &internal_errors.ErrorWithStatusCode{
				Message:    "unsupported file type",
				StatusCode: 400,
			}
		}

		// Act
		_, err := service.Create(testCreationData)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported file type")
	})
}
