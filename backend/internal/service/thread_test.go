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

type MockThreadStorage struct {
	CreateThreadFunc func(title, board string, msg *domain.Message) (int64, error)
	GetThreadFunc    func(id int64) (*domain.Thread, error)
	DeleteThreadFunc func(board string, id int64) error
	ThreadCountFunc  func(board string) (int, error)
	LastThreadIdFunc func(board string) (int64, error)

	mu                 sync.Mutex
	deleteThreadCalled bool
	deleteBoardArg     string
	deleteIdArg        int64
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
	return &domain.Thread{Messages: []*domain.Message{{Id: id}}}, nil
}

func (m *MockThreadStorage) DeleteThread(board string, id int64) error {
	m.mu.Lock()
	m.deleteThreadCalled = true
	m.deleteBoardArg = board
	m.deleteIdArg = id
	m.mu.Unlock()

	if m.DeleteThreadFunc != nil {
		return m.DeleteThreadFunc(board, id)
	}
	return nil
}

func (m *MockThreadStorage) ThreadCount(board string) (int, error) {
	m.mu.Lock()
	m.threadCountCalled = true
	m.mu.Unlock()

	if m.ThreadCountFunc != nil {
		return m.ThreadCountFunc(board)
	}
	return 1, nil
}

func (m *MockThreadStorage) LastThreadId(board string) (int64, error) {
	m.mu.Lock()
	m.getLastIdCalled = true
	m.mu.Unlock()

	if m.LastThreadIdFunc != nil {
		return m.LastThreadIdFunc(board)
	}
	return 0, nil
}

type MockThreadValidator struct {
	TitleFunc func(title string) error
}

func (m *MockThreadValidator) Title(title string) error {
	if m.TitleFunc != nil {
		return m.TitleFunc(title)
	}
	return nil
}

func testConfig(maxThreads *int) config.Public {
	return config.Public{
		MaxThreadCount: maxThreads,
	}
}

func TestThreadCreate(t *testing.T) {
	title := "test title"
	board := "test board"
	msg := &domain.Message{}
	newThreadId := int64(10)
	lastThreadId := int64(1) // Oldest thread ID to be deleted

	t.Run("Successful creation without max thread limit", func(t *testing.T) {
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		cfg := testConfig(nil) // MaxThreadCount is nil
		service := NewThread(storage, validator, cfg)

		storage.CreateThreadFunc = func(tt, b string, m *domain.Message) (int64, error) {
			assert.Equal(t, title, tt)
			assert.Equal(t, board, b)
			return newThreadId, nil
		}

		createdId, err := service.Create(title, board, msg)

		require.NoError(t, err)
		assert.Equal(t, newThreadId, createdId)
		// Assert cleanup methods were NOT called
		storage.mu.Lock() // Lock for reading tracker flags
		assert.False(t, storage.threadCountCalled, "ThreadCount should not be called")
		assert.False(t, storage.getLastIdCalled, "GetLastThreadId should not be called")
		assert.False(t, storage.deleteThreadCalled, "DeleteThread should not be called")
		storage.mu.Unlock()
	})

	t.Run("Successful creation below max thread limit", func(t *testing.T) {
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		maxThreads := 5
		cfg := testConfig(&maxThreads)
		service := NewThread(storage, validator, cfg)

		storage.CreateThreadFunc = func(tt, b string, m *domain.Message) (int64, error) {
			return newThreadId, nil
		}
		storage.ThreadCountFunc = func(b string) (int, error) {
			assert.Equal(t, board, b)
			return maxThreads, nil // Count is exactly the limit (or less)
		}

		createdId, err := service.Create(title, board, msg)

		require.NoError(t, err)
		assert.Equal(t, newThreadId, createdId)
		// Assert cleanup methods (except ThreadCount) were NOT called
		storage.mu.Lock()
		assert.True(t, storage.threadCountCalled, "ThreadCount should be called")
		assert.False(t, storage.getLastIdCalled, "GetLastThreadId should not be called")
		assert.False(t, storage.deleteThreadCalled, "DeleteThread should not be called")
		storage.mu.Unlock()
	})

	t.Run("Successful creation exceeding max limit with successful deletion", func(t *testing.T) {
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		maxThreads := 5
		cfg := testConfig(&maxThreads)
		service := NewThread(storage, validator, cfg)

		storage.CreateThreadFunc = func(tt, b string, m *domain.Message) (int64, error) {
			return newThreadId, nil
		}
		storage.ThreadCountFunc = func(b string) (int, error) {
			assert.Equal(t, board, b)
			return maxThreads + 1, nil // Count exceeds limit
		}
		storage.LastThreadIdFunc = func(b string) (int64, error) {
			assert.Equal(t, board, b)
			return lastThreadId, nil
		}
		storage.DeleteThreadFunc = func(b string, id int64) error {
			assert.Equal(t, board, b)
			assert.Equal(t, lastThreadId, id)
			return nil // Successful deletion
		}

		createdId, err := service.Create(title, board, msg)

		require.NoError(t, err)
		assert.Equal(t, newThreadId, createdId)
		// Assert all relevant cleanup methods were called correctly
		storage.mu.Lock()
		assert.True(t, storage.threadCountCalled, "ThreadCount should be called")
		assert.True(t, storage.getLastIdCalled, "GetLastThreadId should be called")
		assert.True(t, storage.deleteThreadCalled, "DeleteThread should be called")
		assert.Equal(t, board, storage.deleteBoardArg)
		assert.Equal(t, lastThreadId, storage.deleteIdArg)
		storage.mu.Unlock()
	})

	t.Run("Validation error", func(t *testing.T) {
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		cfg := testConfig(nil) // Config doesn't matter here
		service := NewThread(storage, validator, cfg)
		validationError := &internal_errors.ErrorWithStatusCode{Message: "Invalid title", StatusCode: 400}

		validator.TitleFunc = func(t string) error {
			return validationError
		}
		// Ensure CreateThread is not called if validation fails
		createCalled := false
		storage.CreateThreadFunc = func(tt, b string, m *domain.Message) (int64, error) {
			createCalled = true
			return -1, errors.New("should not be called")
		}

		_, err := service.Create(title, board, msg)

		require.Error(t, err)
		assert.Equal(t, validationError, err)
		assert.False(t, createCalled, "CreateThread should not be called on validation error")
		// Assert cleanup methods were NOT called
		storage.mu.Lock()
		assert.False(t, storage.threadCountCalled, "ThreadCount should not be called")
		assert.False(t, storage.getLastIdCalled, "GetLastThreadId should not be called")
		assert.False(t, storage.deleteThreadCalled, "DeleteThread should not be called")
		storage.mu.Unlock()
	})

	t.Run("Storage error during CreateThread", func(t *testing.T) {
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		cfg := testConfig(nil)
		service := NewThread(storage, validator, cfg)
		storageError := errors.New("db connection lost")

		storage.CreateThreadFunc = func(tt, b string, m *domain.Message) (int64, error) {
			return -1, storageError
		}

		_, err := service.Create(title, board, msg)

		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		// Assert cleanup methods were NOT called
		storage.mu.Lock()
		assert.False(t, storage.threadCountCalled, "ThreadCount should not be called")
		assert.False(t, storage.getLastIdCalled, "GetLastThreadId should not be called")
		assert.False(t, storage.deleteThreadCalled, "DeleteThread should not be called")
		storage.mu.Unlock()
	})

	t.Run("Storage error during ThreadCount", func(t *testing.T) {
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		maxThreads := 5
		cfg := testConfig(&maxThreads)
		service := NewThread(storage, validator, cfg)
		storageError := errors.New("count failed")

		storage.CreateThreadFunc = func(tt, b string, m *domain.Message) (int64, error) {
			return newThreadId, nil // Create succeeds
		}
		storage.ThreadCountFunc = func(b string) (int, error) {
			return 0, storageError // Error during count
		}

		createdId, err := service.Create(title, board, msg)

		// IMPORTANT: The thread *was* created, but the cleanup failed.
		// The function should return the created ID *and* the cleanup error.
		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.Equal(t, newThreadId, createdId) // Verify the ID is still returned

		// Assert further cleanup methods were NOT called
		storage.mu.Lock()
		assert.True(t, storage.threadCountCalled, "ThreadCount should be called")
		assert.False(t, storage.getLastIdCalled, "GetLastThreadId should not be called")
		assert.False(t, storage.deleteThreadCalled, "DeleteThread should not be called")
		storage.mu.Unlock()
	})

	t.Run("Storage error during GetLastThreadId", func(t *testing.T) {
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		maxThreads := 5
		cfg := testConfig(&maxThreads)
		service := NewThread(storage, validator, cfg)
		storageError := errors.New("get last id failed")

		storage.CreateThreadFunc = func(tt, b string, m *domain.Message) (int64, error) {
			return newThreadId, nil
		}
		storage.ThreadCountFunc = func(b string) (int, error) {
			return maxThreads + 1, nil // Count exceeds limit
		}
		storage.LastThreadIdFunc = func(b string) (int64, error) {
			return 0, storageError // Error getting last ID
		}

		createdId, err := service.Create(title, board, msg)

		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.Equal(t, newThreadId, createdId) // Verify the ID is still returned

		// Assert DeleteThread was NOT called
		storage.mu.Lock()
		assert.True(t, storage.threadCountCalled, "ThreadCount should be called")
		assert.True(t, storage.getLastIdCalled, "GetLastThreadId should be called")
		assert.False(t, storage.deleteThreadCalled, "DeleteThread should not be called")
		storage.mu.Unlock()
	})

	t.Run("Storage error during DeleteThread", func(t *testing.T) {
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		maxThreads := 5
		cfg := testConfig(&maxThreads)
		service := NewThread(storage, validator, cfg)
		storageError := errors.New("delete failed")

		storage.CreateThreadFunc = func(tt, b string, m *domain.Message) (int64, error) {
			return newThreadId, nil
		}
		storage.ThreadCountFunc = func(b string) (int, error) {
			return maxThreads + 1, nil
		}
		storage.LastThreadIdFunc = func(b string) (int64, error) {
			return lastThreadId, nil
		}
		storage.DeleteThreadFunc = func(b string, id int64) error {
			return storageError // Error during delete
		}

		createdId, err := service.Create(title, board, msg)

		require.Error(t, err)
		assert.True(t, errors.Is(err, storageError))
		assert.Equal(t, newThreadId, createdId) // Verify the ID is still returned

		// Assert all methods up to DeleteThread were called
		storage.mu.Lock()
		assert.True(t, storage.threadCountCalled, "ThreadCount should be called")
		assert.True(t, storage.getLastIdCalled, "GetLastThreadId should be called")
		assert.True(t, storage.deleteThreadCalled, "DeleteThread should be called") // Delete was attempted
		storage.mu.Unlock()
	})
}

// --- Updated TestThreadGet and TestThreadDelete to include config ---

func TestThreadGet(t *testing.T) {
	id := int64(1)
	cfg := testConfig(nil) // Default config

	t.Run("Successful get", func(t *testing.T) {
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		service := NewThread(storage, validator, cfg)
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
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		service := NewThread(storage, validator, cfg)
		mockError := errors.New("mock GetThread error")

		storage.GetThreadFunc = func(i int64) (*domain.Thread, error) {
			return nil, mockError
		}

		_, err := service.Get(id)

		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})
}

func TestThreadDelete(t *testing.T) {
	board := "test board"
	id := int64(1)
	cfg := testConfig(nil) // Default config

	t.Run("Successful deletion", func(t *testing.T) {
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		service := NewThread(storage, validator, cfg)

		deleteCalledCorrectly := false
		storage.DeleteThreadFunc = func(b string, i int64) error {
			assert.Equal(t, board, b)
			assert.Equal(t, id, i)
			deleteCalledCorrectly = true
			return nil
		}

		err := service.Delete(board, id)

		require.NoError(t, err)
		assert.True(t, deleteCalledCorrectly, "DeleteThreadFunc should have been called")
	})

	t.Run("Storage error", func(t *testing.T) {
		storage := &MockThreadStorage{}
		validator := &MockThreadValidator{}
		service := NewThread(storage, validator, cfg)
		mockError := errors.New("mock DeleteThread error")

		storage.DeleteThreadFunc = func(b string, i int64) error {
			return mockError
		}

		err := service.Delete(board, id)

		require.Error(t, err)
		assert.True(t, errors.Is(err, mockError))
	})
}
