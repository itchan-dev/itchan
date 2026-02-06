package service

import (
	"errors"
	"sync" // Used for tracking calls in mocks safely in parallel tests
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

// MockMessageService mocks the MessageService interface.
type MockMessageService struct {
	createFunc func(creationData domain.MessageCreationData) (domain.MsgId, error)
	getFunc    func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error)
	deleteFunc func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) error
}

func (m *MockMessageService) Create(creationData domain.MessageCreationData) (domain.MsgId, error) {
	if m.createFunc != nil {
		return m.createFunc(creationData)
	}
	return 1, nil // Default: return arbitrary ID (always 1 for OP)
}

func (m *MockMessageService) Get(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error) {
	if m.getFunc != nil {
		return m.getFunc(board, threadId, id)
	}
	return domain.Message{}, nil
}

func (m *MockMessageService) Delete(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(board, threadId, id)
	}
	return nil
}

// MockThreadStorage mocks the ThreadStorage interface.
type MockThreadStorage struct {
	createThreadFunc       func(creationData domain.ThreadCreationData) (domain.ThreadId, time.Time, error)
	getThreadFunc          func(board domain.BoardShortName, id domain.ThreadId, page int) (domain.Thread, error)
	deleteThreadFunc       func(board domain.BoardShortName, id domain.ThreadId) error
	threadCountFunc        func(board domain.BoardShortName) (int, error)
	lastThreadIdFunc       func(board domain.BoardShortName) (domain.ThreadId, error)
	togglePinnedStatusFunc func(board domain.BoardShortName, threadId domain.ThreadId) (bool, error)

	mu                 sync.Mutex
	deleteThreadCalled bool
	deleteBoardArg     domain.BoardShortName
	deleteIdArg        domain.ThreadId
	threadCountCalled  bool
	getLastIdCalled    bool
}

func (m *MockThreadStorage) ResetCallTracking() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteThreadCalled = false
	m.deleteBoardArg = ""
	m.deleteIdArg = 0
	m.threadCountCalled = false
	m.getLastIdCalled = false
}

func (m *MockThreadStorage) CreateThread(creationData domain.ThreadCreationData) (domain.ThreadId, time.Time, error) {
	if m.createThreadFunc != nil {
		return m.createThreadFunc(creationData)
	}
	// Default success returns arbitrary ID and current time
	return 1, time.Now().UTC(), nil
}

func (m *MockThreadStorage) GetThread(board domain.BoardShortName, id domain.ThreadId, page int) (domain.Thread, error) {
	if m.getThreadFunc != nil {
		return m.getThreadFunc(board, id, page)
	}
	// Default success returns a basic thread matching the ID in the first message
	return domain.Thread{Messages: []*domain.Message{{MessageMetadata: domain.MessageMetadata{Id: domain.MsgId(id)}}}}, nil
}

func (m *MockThreadStorage) DeleteThread(board domain.BoardShortName, id domain.ThreadId) error {
	m.mu.Lock()
	m.deleteThreadCalled = true
	m.deleteBoardArg = board
	m.deleteIdArg = id
	m.mu.Unlock()

	if m.deleteThreadFunc != nil {
		return m.deleteThreadFunc(board, id)
	}
	return nil // Default success
}

func (m *MockThreadStorage) ThreadCount(board domain.BoardShortName) (int, error) {
	m.mu.Lock()
	m.threadCountCalled = true
	m.mu.Unlock()

	if m.threadCountFunc != nil {
		return m.threadCountFunc(board)
	}
	// Default success returns a plausible count
	return 1, nil
}

func (m *MockThreadStorage) LastThreadId(board domain.BoardShortName) (domain.ThreadId, error) {
	m.mu.Lock()
	m.getLastIdCalled = true
	m.mu.Unlock()

	if m.lastThreadIdFunc != nil {
		return m.lastThreadIdFunc(board)
	}
	// Default success returns an arbitrary old ID (e.g., 0)
	return 0, nil
}

func (m *MockThreadStorage) TogglePinnedStatus(board domain.BoardShortName, threadId domain.ThreadId) (bool, error) {
	if m.togglePinnedStatusFunc != nil {
		return m.togglePinnedStatusFunc(board, threadId)
	}
	return true, nil // Default success, returns new pinned status
}

// MockThreadValidator mocks the ThreadValidator interface.
type MockThreadValidator struct {
	titleFunc func(title domain.ThreadTitle) error
}

func (m *MockThreadValidator) Title(title domain.ThreadTitle) error {
	if m.titleFunc != nil {
		return m.titleFunc(title)
	}
	return nil // Default valid
}

// --- Tests ---

func TestThreadCreate(t *testing.T) {
	// Common test data
	validTitle := domain.ThreadTitle("Test Thread Title")
	validBoard := domain.BoardShortName("tst")
	validOpMessage := domain.MessageCreationData{Text: "This is the OP message"}
	validCreationData := domain.ThreadCreationData{
		Title:     validTitle,
		Board:     validBoard,
		OpMessage: validOpMessage,
	}
	newThreadId := domain.ThreadId(10)

	t.Run("Successful creation", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		messageService := &MockMessageService{}
		service := NewThread(storage, validator, messageService, mediaStorage)
		createCalled := false

		validator.titleFunc = func(title domain.ThreadTitle) error {
			assert.Equal(t, validTitle, title)
			return nil
		}
		storage.createThreadFunc = func(creationData domain.ThreadCreationData) (domain.ThreadId, time.Time, error) {
			createCalled = true
			assert.Equal(t, validCreationData, creationData)
			return newThreadId, time.Now().UTC(), nil
		}
		messageService.createFunc = func(creationData domain.MessageCreationData) (domain.MsgId, error) {
			return 1, nil // OP message always has ID 1
		}

		// Act
		threadId, err := service.Create(validCreationData)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, newThreadId, threadId)
		assert.True(t, createCalled, "Storage CreateThread should be called")
		// Cleanup is now handled by background ThreadGarbageCollector
		storage.mu.Lock()
		assert.False(t, storage.threadCountCalled, "ThreadCount should not be called during creation")
		assert.False(t, storage.getLastIdCalled, "GetLastThreadId should not be called during creation")
		assert.False(t, storage.deleteThreadCalled, "DeleteThread should not be called during creation")
		storage.mu.Unlock()
	})

	t.Run("Validation error", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		messageService := &MockMessageService{}
		service := NewThread(storage, validator, messageService, mediaStorage)
		validationError := &internal_errors.ErrorWithStatusCode{Message: "Invalid title", StatusCode: 400}
		createCalled := false

		validator.titleFunc = func(title domain.ThreadTitle) error {
			assert.Equal(t, validTitle, title)
			return validationError
		}
		storage.createThreadFunc = func(creationData domain.ThreadCreationData) (domain.ThreadId, time.Time, error) {
			createCalled = true
			return -1, time.Time{}, errors.New("should not be called")
		}

		// Act
		_, err := service.Create(validCreationData)

		// Assert
		require.Error(t, err)
		assert.Equal(t, validationError, err)
		assert.False(t, createCalled, "CreateThread should not be called on validation error")
	})

	t.Run("Storage error during CreateThread", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		messageService := &MockMessageService{}
		service := NewThread(storage, validator, messageService, mediaStorage)
		storageError := errors.New("db connection lost")
		createCalled := false

		storage.createThreadFunc = func(creationData domain.ThreadCreationData) (domain.ThreadId, time.Time, error) {
			createCalled = true
			return -1, time.Time{}, storageError
		}

		// Act
		_, err := service.Create(validCreationData)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.True(t, createCalled, "Storage CreateThread should have been attempted")
	})
}

func TestThreadGet(t *testing.T) {
	// Common test data
	testId := domain.ThreadId(1)

	t.Run("Successful get", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{} // Not used in Get
		mediaStorage := &SharedMockMediaStorage{}
		messageService := &MockMessageService{}
		service := NewThread(storage, validator, messageService, mediaStorage)
		expectedThread := domain.Thread{
			ThreadMetadata: domain.ThreadMetadata{Title: "test title"},
			Messages:       []*domain.Message{{MessageMetadata: domain.MessageMetadata{Id: domain.MsgId(testId)}}},
		}
		getCalled := false

		storage.getThreadFunc = func(board domain.BoardShortName, id domain.ThreadId, page int) (domain.Thread, error) {
			getCalled = true
			assert.Equal(t, testId, id)
			assert.Equal(t, 1, page)
			return expectedThread, nil
		}

		// Act
		thread, err := service.Get("test", testId, 1)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedThread, thread)
		assert.True(t, getCalled, "Storage GetThread should be called")
	})

	t.Run("Storage error", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		messageService := &MockMessageService{}
		service := NewThread(storage, validator, messageService, mediaStorage)
		storageError := errors.New("mock GetThread error")
		getCalled := false

		storage.getThreadFunc = func(board domain.BoardShortName, id domain.ThreadId, page int) (domain.Thread, error) {
			getCalled = true
			assert.Equal(t, testId, id)
			return domain.Thread{}, storageError
		}

		// Act
		_, err := service.Get("test", testId, 1)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.True(t, getCalled, "Storage GetThread should be called")
	})
}

func TestThreadDelete(t *testing.T) {
	// Common test data
	testBoard := domain.BoardShortName("tst")
	testId := domain.ThreadId(1)

	t.Run("Successful deletion", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{} // Not used in Delete
		mediaStorage := &SharedMockMediaStorage{}
		messageService := &MockMessageService{}
		service := NewThread(storage, validator, messageService, mediaStorage)

		storage.deleteThreadFunc = func(board domain.BoardShortName, id domain.ThreadId) error {
			assert.Equal(t, testBoard, board)
			assert.Equal(t, testId, id)
			return nil
		}

		// Act
		err := service.Delete(testBoard, testId)

		// Assert
		require.NoError(t, err)
		storage.mu.Lock()
		assert.True(t, storage.deleteThreadCalled, "DeleteThread should have been called")
		assert.Equal(t, testBoard, storage.deleteBoardArg)
		assert.Equal(t, testId, storage.deleteIdArg)
		storage.mu.Unlock()
	})

	t.Run("Storage error", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		messageService := &MockMessageService{}
		service := NewThread(storage, validator, messageService, mediaStorage)
		storageError := errors.New("mock DeleteThread error")

		storage.deleteThreadFunc = func(board domain.BoardShortName, id domain.ThreadId) error {
			assert.Equal(t, testBoard, board)
			assert.Equal(t, testId, id)
			return storageError
		}

		// Act
		err := service.Delete(testBoard, testId)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		storage.mu.Lock()
		assert.True(t, storage.deleteThreadCalled, "DeleteThread should have been called (attempted)")
		assert.Equal(t, testBoard, storage.deleteBoardArg)
		assert.Equal(t, testId, storage.deleteIdArg)
		storage.mu.Unlock()
	})
}

func TestThreadTogglePinned(t *testing.T) {
	// Common test data
	testBoard := domain.BoardShortName("tst")
	testId := domain.ThreadId(42)

	t.Run("Successfully toggle pinned status to true", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		messageService := &MockMessageService{}
		service := NewThread(storage, validator, messageService, mediaStorage)

		toggleCalled := false
		storage.togglePinnedStatusFunc = func(board domain.BoardShortName, threadId domain.ThreadId) (bool, error) {
			toggleCalled = true
			assert.Equal(t, testBoard, board)
			assert.Equal(t, testId, threadId)
			return true, nil // Thread is now pinned
		}

		// Act
		newStatus, err := service.TogglePinned(testBoard, testId)

		// Assert
		require.NoError(t, err)
		assert.True(t, newStatus, "Should return new pinned status as true")
		assert.True(t, toggleCalled, "Storage TogglePinnedStatus should be called")
	})

	t.Run("Successfully toggle pinned status to false", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		messageService := &MockMessageService{}
		service := NewThread(storage, validator, messageService, mediaStorage)

		toggleCalled := false
		storage.togglePinnedStatusFunc = func(board domain.BoardShortName, threadId domain.ThreadId) (bool, error) {
			toggleCalled = true
			assert.Equal(t, testBoard, board)
			assert.Equal(t, testId, threadId)
			return false, nil // Thread is now unpinned
		}

		// Act
		newStatus, err := service.TogglePinned(testBoard, testId)

		// Assert
		require.NoError(t, err)
		assert.False(t, newStatus, "Should return new pinned status as false")
		assert.True(t, toggleCalled, "Storage TogglePinnedStatus should be called")
	})

	t.Run("Storage error during toggle", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		mediaStorage := &SharedMockMediaStorage{}
		messageService := &MockMessageService{}
		service := NewThread(storage, validator, messageService, mediaStorage)

		storageError := errors.New("database connection error")
		toggleCalled := false
		storage.togglePinnedStatusFunc = func(board domain.BoardShortName, threadId domain.ThreadId) (bool, error) {
			toggleCalled = true
			assert.Equal(t, testBoard, board)
			assert.Equal(t, testId, threadId)
			return false, storageError
		}

		// Act
		_, err := service.TogglePinned(testBoard, testId)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.True(t, toggleCalled, "Storage TogglePinnedStatus should have been called")
	})
}
