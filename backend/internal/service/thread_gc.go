package service

import (
	"context"
	"fmt"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
)

// ThreadGarbageCollector handles cleanup of old threads to maintain MaxThreadCount per board.
// It runs periodically to delete the oldest threads when a board exceeds its thread limit.
type ThreadGarbageCollector struct {
	storage          ThreadGCStorage
	threadService    ThreadDeleter
	maxThreadCount   *int
	lastCleanupStats ThreadCleanupStats
}

// ThreadCleanupStats tracks metrics from the last thread cleanup run.
type ThreadCleanupStats struct {
	RunAt          time.Time
	BoardsScanned  int
	BoardsCleaned  int
	ThreadsDeleted int
	DurationMs     int64
	Errors         []string
}

// ThreadGCStorage defines the database operations needed for thread garbage collection.
type ThreadGCStorage interface {
	GetBoards() ([]domain.Board, error)
	ThreadCount(board domain.BoardShortName) (int, error)
	LastThreadId(board domain.BoardShortName) (domain.ThreadId, error)
}

// ThreadDeleter defines the thread deletion operation.
// Using an interface allows us to reuse the existing thread service's Delete method
// which handles both database and filesystem cleanup.
type ThreadDeleter interface {
	Delete(board domain.BoardShortName, id domain.ThreadId) error
}

// NewThreadGarbageCollector creates a new thread garbage collector instance.
// maxThreadCount is the maximum number of threads allowed per board (can be nil to disable cleanup).
func NewThreadGarbageCollector(
	storage ThreadGCStorage,
	threadService ThreadDeleter,
	maxThreadCount *int,
) *ThreadGarbageCollector {
	return &ThreadGarbageCollector{
		storage:        storage,
		threadService:  threadService,
		maxThreadCount: maxThreadCount,
	}
}

// StartBackgroundCleanup starts a background goroutine that runs cleanup periodically.
// It follows the same pattern as MediaGarbageCollector.StartBackgroundCleanup.
func (gc *ThreadGarbageCollector) StartBackgroundCleanup(ctx context.Context, interval time.Duration) {
	// If maxThreadCount is not configured, don't start cleanup
	if gc.maxThreadCount == nil {
		logger.Log.Warn("max thread count not configured, background cleanup disabled",
			"component", "thread_gc")
		return
	}

	ticker := time.NewTicker(interval)
	logger.Log.Info("started thread garbage collector",
		"component", "thread_gc",
		"interval", interval,
		"max_threads_per_board", *gc.maxThreadCount)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := gc.RunCleanup(); err != nil {
					logger.Log.Error("thread gc cleanup failed",
						"component", "thread_gc",
						"error", err)
				} else {
					stats := gc.GetLastCleanupStats()
					logger.Log.Info("thread gc completed",
						"component", "thread_gc",
						"boards_scanned", stats.BoardsScanned,
						"boards_cleaned", stats.BoardsCleaned,
						"threads_deleted", stats.ThreadsDeleted,
						"duration_ms", stats.DurationMs,
						"errors", len(stats.Errors))
				}
			case <-ctx.Done():
				logger.Log.Info("thread gc shutting down gracefully",
					"component", "thread_gc")
				return
			}
		}
	}()
}

// RunCleanup executes a single thread garbage collection cycle.
// It can be called manually for testing or maintenance.
func (gc *ThreadGarbageCollector) RunCleanup() error {
	// If maxThreadCount is not configured, skip cleanup
	if gc.maxThreadCount == nil {
		return nil
	}

	startTime := time.Now()
	stats := ThreadCleanupStats{
		RunAt:  startTime,
		Errors: []string{},
	}

	// Step 1: Get all boards
	boards, err := gc.storage.GetBoards()
	if err != nil {
		return fmt.Errorf("failed to get board list: %w", err)
	}
	stats.BoardsScanned = len(boards)

	// Step 2: For each board, check if cleanup is needed
	for _, board := range boards {
		boardShortName := board.BoardMetadata.ShortName

		// Get current thread count
		threadCount, err := gc.storage.ThreadCount(boardShortName)
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("board '%s': failed to get thread count: %v", boardShortName, err))
			continue
		}

		// If under limit, skip this board
		if threadCount <= *gc.maxThreadCount {
			continue
		}

		// Board needs cleanup
		stats.BoardsCleaned++

		// Delete threads until we're at or below the limit
		// We delete (threadCount - maxThreadCount) threads to get back to the limit
		threadsToDelete := threadCount - *gc.maxThreadCount
		for range threadsToDelete {
			// Get the oldest thread
			lastThreadId, err := gc.storage.LastThreadId(boardShortName)
			if err != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("board '%s': failed to get oldest thread: %v", boardShortName, err))
				break
			}

			// Delete it
			if err := gc.threadService.Delete(boardShortName, lastThreadId); err != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("board '%s': failed to delete thread %d: %v", boardShortName, lastThreadId, err))
				break
			}

			stats.ThreadsDeleted++
		}
	}

	stats.DurationMs = time.Since(startTime).Milliseconds()
	gc.lastCleanupStats = stats

	return nil
}

// GetLastCleanupStats returns statistics from the last cleanup run.
// Useful for monitoring and observability.
func (gc *ThreadGarbageCollector) GetLastCleanupStats() ThreadCleanupStats {
	return gc.lastCleanupStats
}
