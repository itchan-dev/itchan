package service

import (
	"errors"
	"sync" // Used for tracking calls in mocks safely in parallel tests
	"testing"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

// MockThreadStorage mocks the ThreadStorage interface.
type MockThreadStorage struct {
	createThreadFunc func(creationData domain.ThreadCreationData) (domain.MsgId, error)
	getThreadFunc    func(board domain.BoardShortName, id domain.MsgId) (domain.Thread, error)
	deleteThreadFunc func(board domain.BoardShortName, id domain.MsgId) error
	threadCountFunc  func(board domain.BoardShortName) (int, error)
	lastThreadIdFunc func(board domain.BoardShortName) (domain.MsgId, error)

	mu                 sync.Mutex
	deleteThreadCalled bool
	deleteBoardArg     domain.BoardShortName
	deleteIdArg        domain.MsgId
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

func (m *MockThreadStorage) CreateThread(creationData domain.ThreadCreationData) (domain.MsgId, error) {
	if m.createThreadFunc != nil {
		return m.createThreadFunc(creationData)
	}
	// Default success returns an arbitrary ID (e.g., 1)
	return 1, nil
}

func (m *MockThreadStorage) GetThread(board domain.BoardShortName, id domain.MsgId) (domain.Thread, error) {
	if m.getThreadFunc != nil {
		return m.getThreadFunc(board, id)
	}
	// Default success returns a basic thread matching the ID in the first message
	return domain.Thread{Messages: []domain.Message{{MessageMetadata: domain.MessageMetadata{Id: id}}}}, nil
}

func (m *MockThreadStorage) DeleteThread(board domain.BoardShortName, id domain.MsgId) error {
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

func (m *MockThreadStorage) LastThreadId(board domain.BoardShortName) (domain.MsgId, error) {
	m.mu.Lock()
	m.getLastIdCalled = true
	m.mu.Unlock()

	if m.lastThreadIdFunc != nil {
		return m.lastThreadIdFunc(board)
	}
	// Default success returns an arbitrary old ID (e.g., 0)
	return 0, nil
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

// --- Helpers ---

func testConfig(maxThreads *int) config.Public {
	return config.Public{
		MaxThreadCount: maxThreads,
	}
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
	newThreadId := domain.MsgId(10)
	lastThreadId := domain.MsgId(1) // Oldest thread ID assumed to be deleted

	t.Run("Successful creation without max thread limit", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{}
		cfg := testConfig(nil) // MaxThreadCount is nil
		service := NewThread(storage, validator, cfg)
		createCalled := false

		validator.titleFunc = func(title domain.ThreadTitle) error {
			assert.Equal(t, validTitle, title)
			return nil
		}
		storage.createThreadFunc = func(creationData domain.ThreadCreationData) (domain.MsgId, error) {
			createCalled = true
			assert.Equal(t, validCreationData, creationData)
			return newThreadId, nil
		}

		// Act
		createdId, err := service.Create(validCreationData)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, newThreadId, createdId)
		assert.True(t, createCalled, "Storage CreateThread should be called")
		storage.mu.Lock() // Lock for reading tracker flags
		assert.False(t, storage.threadCountCalled, "ThreadCount should not be called")
		assert.False(t, storage.getLastIdCalled, "GetLastThreadId should not be called")
		assert.False(t, storage.deleteThreadCalled, "DeleteThread should not be called")
		storage.mu.Unlock()
	})

	t.Run("Successful creation below max thread limit", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{}
		maxThreads := 5
		cfg := testConfig(&maxThreads)
		service := NewThread(storage, validator, cfg)
		createCalled := false

		storage.createThreadFunc = func(creationData domain.ThreadCreationData) (domain.MsgId, error) {
			createCalled = true
			assert.Equal(t, validCreationData, creationData)
			return newThreadId, nil
		}
		storage.threadCountFunc = func(board domain.BoardShortName) (int, error) {
			assert.Equal(t, validBoard, board)
			return maxThreads - 1, nil // Count is below the limit
		}

		// Act
		createdId, err := service.Create(validCreationData)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, newThreadId, createdId)
		assert.True(t, createCalled, "Storage CreateThread should be called")
		storage.mu.Lock()
		assert.True(t, storage.threadCountCalled, "ThreadCount should be called")
		assert.False(t, storage.getLastIdCalled, "GetLastThreadId should not be called")
		assert.False(t, storage.deleteThreadCalled, "DeleteThread should not be called")
		storage.mu.Unlock()
	})

	t.Run("Successful creation exceeding max limit with successful deletion", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{}
		maxThreads := 5
		cfg := testConfig(&maxThreads)
		service := NewThread(storage, validator, cfg)
		createCalled := false

		storage.createThreadFunc = func(creationData domain.ThreadCreationData) (domain.MsgId, error) {
			createCalled = true
			return newThreadId, nil
		}
		storage.threadCountFunc = func(board domain.BoardShortName) (int, error) {
			assert.Equal(t, validBoard, board)
			return maxThreads + 1, nil // Count exceeds limit
		}
		storage.lastThreadIdFunc = func(board domain.BoardShortName) (domain.MsgId, error) {
			assert.Equal(t, validBoard, board)
			return lastThreadId, nil
		}
		// DeleteThreadFunc uses the mock's built-in tracking

		// Act
		createdId, err := service.Create(validCreationData)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, newThreadId, createdId)
		assert.True(t, createCalled, "Storage CreateThread should be called")
		storage.mu.Lock()
		assert.True(t, storage.threadCountCalled, "ThreadCount should be called")
		assert.True(t, storage.getLastIdCalled, "GetLastThreadId should be called")
		assert.True(t, storage.deleteThreadCalled, "DeleteThread should be called")
		assert.Equal(t, validBoard, storage.deleteBoardArg)
		assert.Equal(t, lastThreadId, storage.deleteIdArg)
		storage.mu.Unlock()
	})

	t.Run("Validation error", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{}
		cfg := testConfig(nil) // Config doesn't matter here
		service := NewThread(storage, validator, cfg)
		validationError := &internal_errors.ErrorWithStatusCode{Message: "Invalid title", StatusCode: 400}
		createCalled := false

		validator.titleFunc = func(title domain.ThreadTitle) error {
			assert.Equal(t, validTitle, title)
			return validationError
		}
		storage.createThreadFunc = func(creationData domain.ThreadCreationData) (domain.MsgId, error) {
			createCalled = true
			return -1, errors.New("should not be called")
		}

		// Act
		_, err := service.Create(validCreationData)

		// Assert
		require.Error(t, err)
		assert.Equal(t, validationError, err) // Check for the specific validation error instance
		assert.False(t, createCalled, "CreateThread should not be called on validation error")
		storage.mu.Lock()
		assert.False(t, storage.threadCountCalled, "ThreadCount should not be called")
		assert.False(t, storage.getLastIdCalled, "GetLastThreadId should not be called")
		assert.False(t, storage.deleteThreadCalled, "DeleteThread should not be called")
		storage.mu.Unlock()
	})

	t.Run("Storage error during CreateThread", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{}
		cfg := testConfig(nil)
		service := NewThread(storage, validator, cfg)
		storageError := errors.New("db connection lost")
		createCalled := false

		storage.createThreadFunc = func(creationData domain.ThreadCreationData) (domain.MsgId, error) {
			createCalled = true
			return -1, storageError
		}

		// Act
		_, err := service.Create(validCreationData)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.True(t, createCalled, "Storage CreateThread should have been attempted")
		storage.mu.Lock()
		assert.False(t, storage.threadCountCalled, "ThreadCount should not be called")
		assert.False(t, storage.getLastIdCalled, "GetLastThreadId should not be called")
		assert.False(t, storage.deleteThreadCalled, "DeleteThread should not be called")
		storage.mu.Unlock()
	})

	t.Run("Storage error during ThreadCount", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{}
		maxThreads := 5
		cfg := testConfig(&maxThreads)
		service := NewThread(storage, validator, cfg)
		storageError := errors.New("count failed")
		createCalled := false

		storage.createThreadFunc = func(creationData domain.ThreadCreationData) (domain.MsgId, error) {
			createCalled = true
			return newThreadId, nil // Create succeeds
		}
		storage.threadCountFunc = func(board domain.BoardShortName) (int, error) {
			assert.Equal(t, validBoard, board)
			return 0, storageError // Error during count
		}

		// Act
		createdId, err := service.Create(validCreationData)

		// Assert
		// IMPORTANT: The thread *was* created, but the cleanup failed.
		// The function should return the created ID *and* the cleanup error.
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.Equal(t, newThreadId, createdId, "Created thread ID should still be returned on cleanup error")
		assert.True(t, createCalled, "Storage CreateThread should be called")
		storage.mu.Lock()
		assert.True(t, storage.threadCountCalled, "ThreadCount should be called")
		assert.False(t, storage.getLastIdCalled, "GetLastThreadId should not be called")
		assert.False(t, storage.deleteThreadCalled, "DeleteThread should not be called")
		storage.mu.Unlock()
	})

	t.Run("Storage error during GetLastThreadId", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{}
		maxThreads := 5
		cfg := testConfig(&maxThreads)
		service := NewThread(storage, validator, cfg)
		storageError := errors.New("get last id failed")
		createCalled := false

		storage.createThreadFunc = func(creationData domain.ThreadCreationData) (domain.MsgId, error) {
			createCalled = true
			return newThreadId, nil
		}
		storage.threadCountFunc = func(board domain.BoardShortName) (int, error) {
			assert.Equal(t, validBoard, board)
			return maxThreads + 1, nil // Count exceeds limit
		}
		storage.lastThreadIdFunc = func(board domain.BoardShortName) (domain.MsgId, error) {
			assert.Equal(t, validBoard, board)
			return 0, storageError // Error getting last ID
		}

		// Act
		createdId, err := service.Create(validCreationData)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.Equal(t, newThreadId, createdId, "Created thread ID should still be returned on cleanup error")
		assert.True(t, createCalled, "Storage CreateThread should be called")
		storage.mu.Lock()
		assert.True(t, storage.threadCountCalled, "ThreadCount should be called")
		assert.True(t, storage.getLastIdCalled, "GetLastThreadId should be called")
		assert.False(t, storage.deleteThreadCalled, "DeleteThread should not be called")
		storage.mu.Unlock()
	})

	t.Run("Storage error during DeleteThread", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{}
		maxThreads := 5
		cfg := testConfig(&maxThreads)
		service := NewThread(storage, validator, cfg)
		storageError := errors.New("delete failed")
		createCalled := false

		storage.createThreadFunc = func(creationData domain.ThreadCreationData) (domain.MsgId, error) {
			createCalled = true
			return newThreadId, nil
		}
		storage.threadCountFunc = func(board domain.BoardShortName) (int, error) {
			return maxThreads + 1, nil
		}
		storage.lastThreadIdFunc = func(board domain.BoardShortName) (domain.MsgId, error) {
			return lastThreadId, nil
		}
		// DeleteThreadFunc uses the mock's built-in tracking but returns an error
		storage.deleteThreadFunc = func(b domain.BoardShortName, id domain.MsgId) error {
			assert.Equal(t, validBoard, b)
			assert.Equal(t, lastThreadId, id)
			return storageError // Error during delete
		}

		// Act
		createdId, err := service.Create(validCreationData)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.Equal(t, newThreadId, createdId, "Created thread ID should still be returned on cleanup error")
		assert.True(t, createCalled, "Storage CreateThread should be called")
		storage.mu.Lock()
		assert.True(t, storage.threadCountCalled, "ThreadCount should be called")
		assert.True(t, storage.getLastIdCalled, "GetLastThreadId should be called")
		assert.True(t, storage.deleteThreadCalled, "DeleteThread should be called (attempted)")
		assert.Equal(t, validBoard, storage.deleteBoardArg) // Verify args even on failure
		assert.Equal(t, lastThreadId, storage.deleteIdArg)
		storage.mu.Unlock()
	})
}

func TestThreadGet(t *testing.T) {
	// Common test data
	testId := domain.MsgId(1)
	cfg := testConfig(nil) // Default config, not relevant for Get

	t.Run("Successful get", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{} // Not used in Get
		service := NewThread(storage, validator, cfg)
		// Use domain types consistently
		expectedThread := domain.Thread{
			ThreadMetadata: domain.ThreadMetadata{Title: "test title"},
			Messages:       []domain.Message{{MessageMetadata: domain.MessageMetadata{Id: testId}}},
		}
		getCalled := false

		storage.getThreadFunc = func(board domain.BoardShortName, id domain.MsgId) (domain.Thread, error) {
			getCalled = true
			assert.Equal(t, testId, id)
			return expectedThread, nil
		}

		// Act
		thread, err := service.Get("test", testId)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedThread, thread) // Compare the whole struct
		assert.True(t, getCalled, "Storage GetThread should be called")
	})

	t.Run("Storage error", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		service := NewThread(storage, validator, cfg)
		storageError := errors.New("mock GetThread error")
		getCalled := false

		storage.getThreadFunc = func(board domain.BoardShortName, id domain.MsgId) (domain.Thread, error) {
			getCalled = true
			assert.Equal(t, testId, id)
			return domain.Thread{}, storageError
		}

		// Act
		_, err := service.Get("test", testId)

		// Assert
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.True(t, getCalled, "Storage GetThread should be called")
	})
}

func TestThreadDelete(t *testing.T) {
	// Common test data
	testBoard := domain.BoardShortName("tst")
	testId := domain.MsgId(1)
	cfg := testConfig(nil) // Default config, not relevant for Delete

	t.Run("Successful deletion", func(t *testing.T) {
		// Arrange
		storage := &MockThreadStorage{}
		storage.ResetCallTracking()
		validator := &MockThreadValidator{} // Not used in Delete
		service := NewThread(storage, validator, cfg)

		// Use built-in mock tracking
		storage.deleteThreadFunc = func(board domain.BoardShortName, id domain.MsgId) error {
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
		service := NewThread(storage, validator, cfg)
		storageError := errors.New("mock DeleteThread error")

		// Use built-in mock tracking
		storage.deleteThreadFunc = func(board domain.BoardShortName, id domain.MsgId) error {
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
		assert.Equal(t, testBoard, storage.deleteBoardArg) // Verify args even on failure
		assert.Equal(t, testId, storage.deleteIdArg)
		storage.mu.Unlock()
	})
}
