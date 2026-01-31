package service

import (
	"errors"
	"sync"
	"testing"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mocks for Thread GC Tests ---

type MockThreadGCStorage struct {
	mu               sync.Mutex
	getBoardsFunc    func() ([]domain.Board, error)
	threadCountFunc  func(board domain.BoardShortName) (int, error)
	lastThreadIdFunc func(board domain.BoardShortName) (domain.ThreadId, error)
	getBoardsCalls   int
	threadCountCalls map[string]int
	lastThreadCalls  map[string]int
}

func (m *MockThreadGCStorage) GetBoards() ([]domain.Board, error) {
	m.mu.Lock()
	m.getBoardsCalls++
	m.mu.Unlock()

	if m.getBoardsFunc != nil {
		return m.getBoardsFunc()
	}
	return []domain.Board{}, nil
}

func (m *MockThreadGCStorage) ThreadCount(board domain.BoardShortName) (int, error) {
	m.mu.Lock()
	if m.threadCountCalls == nil {
		m.threadCountCalls = make(map[string]int)
	}
	m.threadCountCalls[string(board)]++
	m.mu.Unlock()

	if m.threadCountFunc != nil {
		return m.threadCountFunc(board)
	}
	return 0, nil
}

func (m *MockThreadGCStorage) LastThreadId(board domain.BoardShortName) (domain.ThreadId, error) {
	m.mu.Lock()
	if m.lastThreadCalls == nil {
		m.lastThreadCalls = make(map[string]int)
	}
	m.lastThreadCalls[string(board)]++
	m.mu.Unlock()

	if m.lastThreadIdFunc != nil {
		return m.lastThreadIdFunc(board)
	}
	return 0, nil
}

type MockThreadDeleter struct {
	mu          sync.Mutex
	deleteFunc  func(board domain.BoardShortName, id domain.ThreadId) error
	deleteCalls []struct {
		Board domain.BoardShortName
		Id    domain.ThreadId
	}
}

func (m *MockThreadDeleter) Delete(board domain.BoardShortName, id domain.ThreadId) error {
	m.mu.Lock()
	m.deleteCalls = append(m.deleteCalls, struct {
		Board domain.BoardShortName
		Id    domain.ThreadId
	}{Board: board, Id: id})
	m.mu.Unlock()

	if m.deleteFunc != nil {
		return m.deleteFunc(board, id)
	}
	return nil
}

// --- Tests ---

func TestThreadGarbageCollectorCleanup(t *testing.T) {
	t.Run("successfully cleans up threads when over limit", func(t *testing.T) {
		storage := &MockThreadGCStorage{}
		deleter := &MockThreadDeleter{}
		maxThreads := 10

		// Mock boards
		boards := []domain.Board{
			{BoardMetadata: domain.BoardMetadata{ShortName: "tech"}},
			{BoardMetadata: domain.BoardMetadata{ShortName: "dev"}},
		}
		storage.getBoardsFunc = func() ([]domain.Board, error) {
			return boards, nil
		}

		// Board "tech" has 15 threads (5 over limit) - should delete 5
		// Board "dev" has 8 threads (under limit) - should delete 0
		threadCounts := map[string]int{
			"tech": 15,
			"dev":  8,
		}
		storage.threadCountFunc = func(board domain.BoardShortName) (int, error) {
			return threadCounts[string(board)], nil
		}

		// Mock LastThreadId to return sequential IDs
		// For "tech" board, return thread IDs: 101, 102, 103, 104, 105
		lastThreadIds := map[string][]domain.ThreadId{
			"tech": {101, 102, 103, 104, 105},
		}
		lastThreadCallCount := map[string]int{}
		storage.lastThreadIdFunc = func(board domain.BoardShortName) (domain.ThreadId, error) {
			boardName := string(board)
			count := lastThreadCallCount[boardName]
			lastThreadCallCount[boardName]++
			if ids, ok := lastThreadIds[boardName]; ok && count < len(ids) {
				return ids[count], nil
			}
			return 0, errors.New("no more threads")
		}

		gc := NewThreadGarbageCollector(storage, deleter, &maxThreads)

		// Run cleanup
		err := gc.RunCleanup()
		require.NoError(t, err)

		// Verify stats
		stats := gc.GetLastCleanupStats()
		assert.Equal(t, 2, stats.BoardsScanned, "Should scan 2 boards")
		assert.Equal(t, 1, stats.BoardsCleaned, "Should clean 1 board (tech)")
		assert.Equal(t, 5, stats.ThreadsDeleted, "Should delete 5 threads from tech")
		assert.Empty(t, stats.Errors, "Should have no errors")

		// Verify correct threads were deleted
		deleter.mu.Lock()
		assert.Len(t, deleter.deleteCalls, 5, "Should delete 5 threads")
		for i, call := range deleter.deleteCalls {
			assert.Equal(t, domain.BoardShortName("tech"), call.Board)
			assert.Equal(t, lastThreadIds["tech"][i], call.Id)
		}
		deleter.mu.Unlock()
	})

	t.Run("no cleanup when under limit", func(t *testing.T) {
		storage := &MockThreadGCStorage{}
		deleter := &MockThreadDeleter{}
		maxThreads := 10

		// Mock boards
		boards := []domain.Board{
			{BoardMetadata: domain.BoardMetadata{ShortName: "tech"}},
			{BoardMetadata: domain.BoardMetadata{ShortName: "dev"}},
		}
		storage.getBoardsFunc = func() ([]domain.Board, error) {
			return boards, nil
		}

		// Both boards under limit
		storage.threadCountFunc = func(board domain.BoardShortName) (int, error) {
			return 5, nil // Under limit of 10
		}

		gc := NewThreadGarbageCollector(storage, deleter, &maxThreads)

		// Run cleanup
		err := gc.RunCleanup()
		require.NoError(t, err)

		// Verify stats
		stats := gc.GetLastCleanupStats()
		assert.Equal(t, 2, stats.BoardsScanned)
		assert.Equal(t, 0, stats.BoardsCleaned, "Should clean 0 boards")
		assert.Equal(t, 0, stats.ThreadsDeleted, "Should delete 0 threads")
		assert.Empty(t, stats.Errors)

		// Verify no delete calls
		deleter.mu.Lock()
		assert.Empty(t, deleter.deleteCalls, "Should not delete any threads")
		deleter.mu.Unlock()
	})

	t.Run("cleanup disabled when maxThreadCount is nil", func(t *testing.T) {
		storage := &MockThreadGCStorage{}
		deleter := &MockThreadDeleter{}

		gc := NewThreadGarbageCollector(storage, deleter, nil)

		// Run cleanup
		err := gc.RunCleanup()
		require.NoError(t, err)

		// Verify no operations were performed
		storage.mu.Lock()
		assert.Equal(t, 0, storage.getBoardsCalls, "Should not call GetBoards when disabled")
		storage.mu.Unlock()

		deleter.mu.Lock()
		assert.Empty(t, deleter.deleteCalls, "Should not delete any threads")
		deleter.mu.Unlock()
	})

	t.Run("handles errors gracefully and continues cleanup", func(t *testing.T) {
		storage := &MockThreadGCStorage{}
		deleter := &MockThreadDeleter{}
		maxThreads := 10

		// Mock boards
		boards := []domain.Board{
			{BoardMetadata: domain.BoardMetadata{ShortName: "tech"}},
			{BoardMetadata: domain.BoardMetadata{ShortName: "dev"}},
			{BoardMetadata: domain.BoardMetadata{ShortName: "random"}},
		}
		storage.getBoardsFunc = func() ([]domain.Board, error) {
			return boards, nil
		}

		// Mock thread counts with one error
		storage.threadCountFunc = func(board domain.BoardShortName) (int, error) {
			if board == "dev" {
				return 0, errors.New("database error")
			}
			if board == "tech" {
				return 12, nil // 2 over limit
			}
			return 5, nil // random board - under limit
		}

		// Mock LastThreadId for tech board
		lastThreadCallCount := 0
		storage.lastThreadIdFunc = func(board domain.BoardShortName) (domain.ThreadId, error) {
			if board == "tech" {
				lastThreadCallCount++
				if lastThreadCallCount == 2 {
					return 0, errors.New("failed to get thread id")
				}
				return domain.ThreadId(100 + lastThreadCallCount), nil
			}
			return 0, errors.New("unexpected board")
		}

		// Mock delete with one failure
		deleter.deleteFunc = func(board domain.BoardShortName, id domain.ThreadId) error {
			if id == 101 {
				return errors.New("delete failed")
			}
			return nil
		}

		gc := NewThreadGarbageCollector(storage, deleter, &maxThreads)

		// Run cleanup (should not fail despite errors)
		err := gc.RunCleanup()
		require.NoError(t, err)

		// Verify stats track errors
		stats := gc.GetLastCleanupStats()
		assert.Equal(t, 3, stats.BoardsScanned)
		assert.Equal(t, 1, stats.BoardsCleaned, "Should attempt to clean tech board")
		assert.Len(t, stats.Errors, 2, "Should record 2 errors")

		// Check that both errors are present (order doesn't matter)
		allErrors := stats.Errors[0] + stats.Errors[1]
		assert.Contains(t, allErrors, "database error", "Should track thread count error")
		assert.Contains(t, allErrors, "delete failed", "Should track delete error")

		// Verify one delete attempt was made (failed)
		deleter.mu.Lock()
		assert.Len(t, deleter.deleteCalls, 1, "Should attempt to delete 1 thread")
		deleter.mu.Unlock()
	})

	t.Run("handles empty board list", func(t *testing.T) {
		storage := &MockThreadGCStorage{}
		deleter := &MockThreadDeleter{}
		maxThreads := 10

		// Empty board list
		storage.getBoardsFunc = func() ([]domain.Board, error) {
			return []domain.Board{}, nil
		}

		gc := NewThreadGarbageCollector(storage, deleter, &maxThreads)

		// Run cleanup
		err := gc.RunCleanup()
		require.NoError(t, err)

		// Verify stats
		stats := gc.GetLastCleanupStats()
		assert.Equal(t, 0, stats.BoardsScanned)
		assert.Equal(t, 0, stats.BoardsCleaned)
		assert.Equal(t, 0, stats.ThreadsDeleted)
		assert.Empty(t, stats.Errors)

		// Verify no delete calls
		deleter.mu.Lock()
		assert.Empty(t, deleter.deleteCalls)
		deleter.mu.Unlock()
	})

	t.Run("handles GetBoards error", func(t *testing.T) {
		storage := &MockThreadGCStorage{}
		deleter := &MockThreadDeleter{}
		maxThreads := 10

		// GetBoards fails
		storage.getBoardsFunc = func() ([]domain.Board, error) {
			return nil, errors.New("connection timeout")
		}

		gc := NewThreadGarbageCollector(storage, deleter, &maxThreads)

		// Run cleanup should return error
		err := gc.RunCleanup()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get board list")
		assert.Contains(t, err.Error(), "connection timeout")

		// Verify no delete calls
		deleter.mu.Lock()
		assert.Empty(t, deleter.deleteCalls)
		deleter.mu.Unlock()
	})
}

func TestNewThreadGarbageCollector(t *testing.T) {
	storage := &MockThreadGCStorage{}
	deleter := &MockThreadDeleter{}
	maxThreads := 100

	gc := NewThreadGarbageCollector(storage, deleter, &maxThreads)

	assert.NotNil(t, gc)
	assert.Equal(t, storage, gc.storage)
	assert.Equal(t, deleter, gc.threadService)
	assert.Equal(t, &maxThreads, gc.maxThreadCount)
}

func TestGetLastCleanupStats(t *testing.T) {
	storage := &MockThreadGCStorage{}
	deleter := &MockThreadDeleter{}
	maxThreads := 10

	gc := NewThreadGarbageCollector(storage, deleter, &maxThreads)

	// Initially, stats should be zero
	stats := gc.GetLastCleanupStats()
	assert.Equal(t, 0, stats.BoardsScanned)
	assert.Equal(t, 0, stats.ThreadsDeleted)

	// Run cleanup
	boards := []domain.Board{
		{BoardMetadata: domain.BoardMetadata{ShortName: "tech"}},
	}
	storage.getBoardsFunc = func() ([]domain.Board, error) {
		return boards, nil
	}
	storage.threadCountFunc = func(board domain.BoardShortName) (int, error) {
		return 5, nil // Under limit
	}

	err := gc.RunCleanup()
	require.NoError(t, err)

	// Verify stats are updated
	stats = gc.GetLastCleanupStats()
	assert.Equal(t, 1, stats.BoardsScanned)
	assert.GreaterOrEqual(t, stats.DurationMs, int64(0), "Duration should be >= 0")
}
