package service

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mocks for GC Tests ---

type MockGCStorage struct {
	mu                            sync.Mutex
	getAllFilePathsFunc           func() ([]string, error)
	deleteOrphanedFileRecordsFunc func() (int64, error)
	getAllFilePathsCalls          int
	deleteOrphanedRecordsCalls    int
}

func (m *MockGCStorage) GetAllFilePaths() ([]string, error) {
	m.mu.Lock()
	m.getAllFilePathsCalls++
	m.mu.Unlock()

	if m.getAllFilePathsFunc != nil {
		return m.getAllFilePathsFunc()
	}
	return []string{}, nil
}

func (m *MockGCStorage) DeleteOrphanedFileRecords() (int64, error) {
	m.mu.Lock()
	m.deleteOrphanedRecordsCalls++
	m.mu.Unlock()

	if m.deleteOrphanedFileRecordsFunc != nil {
		return m.deleteOrphanedFileRecordsFunc()
	}
	return 0, nil
}

type MockGCMediaStorage struct {
	mu                  sync.Mutex
	walkFilesFunc       func() ([]string, error)
	getFileModTimeFunc  func(filePath string) (time.Time, error)
	deleteFileFunc      func(filePath string) error
	walkFilesCalls      int
	getFileModTimeCalls map[string]int
	deleteFileCalls     []string
}

func (m *MockGCMediaStorage) WalkFiles() ([]string, error) {
	m.mu.Lock()
	m.walkFilesCalls++
	m.mu.Unlock()

	if m.walkFilesFunc != nil {
		return m.walkFilesFunc()
	}
	return []string{}, nil
}

func (m *MockGCMediaStorage) GetFileModTime(filePath string) (time.Time, error) {
	m.mu.Lock()
	if m.getFileModTimeCalls == nil {
		m.getFileModTimeCalls = make(map[string]int)
	}
	m.getFileModTimeCalls[filePath]++
	m.mu.Unlock()

	if m.getFileModTimeFunc != nil {
		return m.getFileModTimeFunc(filePath)
	}
	// Default: return old timestamp (well past safety threshold)
	return time.Now().Add(-1 * time.Hour), nil
}

func (m *MockGCMediaStorage) DeleteFile(filePath string) error {
	m.mu.Lock()
	m.deleteFileCalls = append(m.deleteFileCalls, filePath)
	m.mu.Unlock()

	if m.deleteFileFunc != nil {
		return m.deleteFileFunc(filePath)
	}
	return nil
}

// --- Tests ---

func TestMediaGarbageCollectorCleanup(t *testing.T) {
	t.Run("successfully cleans up orphaned files from DB and disk", func(t *testing.T) {
		storage := &MockGCStorage{}
		mediaStorage := &MockGCMediaStorage{}
		safetyThreshold := 5 * time.Minute

		// Mock database file paths (files that ARE referenced in DB)
		dbFilePaths := []string{
			"tech/1/image1.jpg",
			"tech/1/image2.png",
			"tech/2/video1.mp4",
		}
		storage.getAllFilePathsFunc = func() ([]string, error) {
			return dbFilePaths, nil
		}

		// Mock orphaned file records in DB (2 records to delete)
		storage.deleteOrphanedFileRecordsFunc = func() (int64, error) {
			return 2, nil
		}

		// Mock filesystem files (includes both DB files and orphaned files)
		fsFiles := []string{
			"tech/1/image1.jpg",       // In DB - keep
			"tech/1/image2.png",       // In DB - keep
			"tech/2/video1.mp4",       // In DB - keep
			"tech/1/orphan1.jpg",      // NOT in DB - delete
			"tech/3/orphan2.webm",     // NOT in DB - delete
			"tech/1/thumb_orphan.jpg", // NOT in DB - delete
		}
		mediaStorage.walkFilesFunc = func() ([]string, error) {
			return fsFiles, nil
		}

		// All files are old enough to delete (past safety threshold)
		mediaStorage.getFileModTimeFunc = func(filePath string) (time.Time, error) {
			return time.Now().Add(-10 * time.Minute), nil
		}

		gc := NewMediaGarbageCollector(storage, mediaStorage, safetyThreshold)

		// Run cleanup
		err := gc.RunCleanup()
		require.NoError(t, err)

		// Verify DB orphaned records were deleted first
		storage.mu.Lock()
		assert.Equal(t, 1, storage.deleteOrphanedRecordsCalls, "Should delete orphaned DB records")
		storage.mu.Unlock()

		// Verify stats
		stats := gc.GetLastCleanupStats()
		assert.Equal(t, 6, stats.FilesScanned, "Should scan all 6 files")
		assert.Equal(t, 3, stats.OrphanedFiles, "Should detect 3 orphaned files")
		assert.Equal(t, 3, stats.FilesDeleted, "Should delete 3 orphaned files")
		assert.Equal(t, 2, stats.FileRecordsDeleted, "Should delete 2 orphaned DB records")
		assert.Empty(t, stats.Errors, "Should have no errors")

		// Verify correct files were deleted from disk
		mediaStorage.mu.Lock()
		assert.Len(t, mediaStorage.deleteFileCalls, 3, "Should delete 3 orphaned files")
		assert.Contains(t, mediaStorage.deleteFileCalls, "tech/1/orphan1.jpg")
		assert.Contains(t, mediaStorage.deleteFileCalls, "tech/3/orphan2.webm")
		assert.Contains(t, mediaStorage.deleteFileCalls, "tech/1/thumb_orphan.jpg")
		assert.NotContains(t, mediaStorage.deleteFileCalls, "tech/1/image1.jpg", "Should NOT delete DB-referenced file")
		mediaStorage.mu.Unlock()
	})

	t.Run("respects safety threshold and skips young files", func(t *testing.T) {
		storage := &MockGCStorage{}
		mediaStorage := &MockGCMediaStorage{}
		safetyThreshold := 5 * time.Minute

		// No files in DB (all filesystem files are orphans)
		storage.getAllFilePathsFunc = func() ([]string, error) {
			return []string{}, nil
		}

		storage.deleteOrphanedFileRecordsFunc = func() (int64, error) {
			return 0, nil
		}

		// Mock filesystem with orphaned files of different ages
		fsFiles := []string{
			"tech/1/old_orphan.jpg",   // Old - should delete
			"tech/1/young_orphan.jpg", // Young - should skip
		}
		mediaStorage.walkFilesFunc = func() ([]string, error) {
			return fsFiles, nil
		}

		// Mock different file ages
		mediaStorage.getFileModTimeFunc = func(filePath string) (time.Time, error) {
			if filePath == "tech/1/old_orphan.jpg" {
				return time.Now().Add(-10 * time.Minute), nil // Old enough
			}
			return time.Now().Add(-1 * time.Minute), nil // Too young
		}

		gc := NewMediaGarbageCollector(storage, mediaStorage, safetyThreshold)

		// Run cleanup
		err := gc.RunCleanup()
		require.NoError(t, err)

		// Verify stats
		stats := gc.GetLastCleanupStats()
		assert.Equal(t, 2, stats.FilesScanned, "Should scan 2 files")
		assert.Equal(t, 1, stats.OrphanedFiles, "Should detect 1 orphan (1 too young)")
		assert.Equal(t, 1, stats.FilesDeleted, "Should delete only 1 old orphan")

		// Verify only old file was deleted
		mediaStorage.mu.Lock()
		assert.Len(t, mediaStorage.deleteFileCalls, 1)
		assert.Contains(t, mediaStorage.deleteFileCalls, "tech/1/old_orphan.jpg")
		assert.NotContains(t, mediaStorage.deleteFileCalls, "tech/1/young_orphan.jpg", "Should NOT delete young file")
		mediaStorage.mu.Unlock()
	})

	t.Run("handles errors gracefully and continues cleanup", func(t *testing.T) {
		storage := &MockGCStorage{}
		mediaStorage := &MockGCMediaStorage{}
		safetyThreshold := 5 * time.Minute

		// DB operations fail
		storage.deleteOrphanedFileRecordsFunc = func() (int64, error) {
			return 0, errors.New("database connection error")
		}

		storage.getAllFilePathsFunc = func() ([]string, error) {
			return []string{"tech/1/image1.jpg"}, nil
		}

		fsFiles := []string{
			"tech/1/image1.jpg",  // In DB - keep
			"tech/1/orphan1.jpg", // Orphan - delete
			"tech/1/orphan2.jpg", // Orphan - delete fails
		}
		mediaStorage.walkFilesFunc = func() ([]string, error) {
			return fsFiles, nil
		}

		mediaStorage.getFileModTimeFunc = func(filePath string) (time.Time, error) {
			return time.Now().Add(-10 * time.Minute), nil
		}

		// Mock delete failure for one file
		mediaStorage.deleteFileFunc = func(filePath string) error {
			if filePath == "tech/1/orphan2.jpg" {
				return errors.New("permission denied")
			}
			return nil
		}

		gc := NewMediaGarbageCollector(storage, mediaStorage, safetyThreshold)

		// Run cleanup (should not fail despite errors)
		err := gc.RunCleanup()
		require.NoError(t, err)

		// Verify stats track errors
		stats := gc.GetLastCleanupStats()
		assert.Equal(t, 3, stats.FilesScanned)
		assert.Equal(t, 2, stats.OrphanedFiles, "Should detect 2 orphaned files")
		assert.Equal(t, 1, stats.FilesDeleted, "Should successfully delete 1 file")
		assert.Len(t, stats.Errors, 2, "Should record 2 errors (DB + file delete)")
		assert.Contains(t, stats.Errors[0], "database connection error")
		assert.Contains(t, stats.Errors[1], "permission denied")

		// Verify one file was successfully deleted
		mediaStorage.mu.Lock()
		assert.Len(t, mediaStorage.deleteFileCalls, 2, "Should attempt to delete 2 files")
		assert.Contains(t, mediaStorage.deleteFileCalls, "tech/1/orphan1.jpg")
		assert.Contains(t, mediaStorage.deleteFileCalls, "tech/1/orphan2.jpg")
		mediaStorage.mu.Unlock()
	})

	t.Run("handles empty filesystem and database", func(t *testing.T) {
		storage := &MockGCStorage{}
		mediaStorage := &MockGCMediaStorage{}
		safetyThreshold := 5 * time.Minute

		// Empty database and filesystem
		storage.getAllFilePathsFunc = func() ([]string, error) {
			return []string{}, nil
		}

		storage.deleteOrphanedFileRecordsFunc = func() (int64, error) {
			return 0, nil
		}

		mediaStorage.walkFilesFunc = func() ([]string, error) {
			return []string{}, nil
		}

		gc := NewMediaGarbageCollector(storage, mediaStorage, safetyThreshold)

		// Run cleanup
		err := gc.RunCleanup()
		require.NoError(t, err)

		// Verify stats
		stats := gc.GetLastCleanupStats()
		assert.Equal(t, 0, stats.FilesScanned)
		assert.Equal(t, 0, stats.OrphanedFiles)
		assert.Equal(t, 0, stats.FilesDeleted)
		assert.Equal(t, 0, stats.FileRecordsDeleted)
		assert.Empty(t, stats.Errors)

		// Verify no delete calls
		mediaStorage.mu.Lock()
		assert.Empty(t, mediaStorage.deleteFileCalls)
		mediaStorage.mu.Unlock()
	})
}
