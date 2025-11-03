package service

import (
	"context"
	"log"
	"path/filepath"
	"time"
)

// MediaGarbageCollector handles cleanup of orphaned media files.
// It compares files on disk with database records and removes orphans.
type MediaGarbageCollector struct {
	storage          GCStorage
	mediaStorage     GCMediaStorage
	safetyThreshold  time.Duration
	lastCleanupStats CleanupStats
}

// CleanupStats tracks metrics from the last garbage collection run.
type CleanupStats struct {
	RunAt              time.Time
	FilesScanned       int
	OrphanedFiles      int
	FilesDeleted       int
	BytesReclaimed     int64
	DurationMs         int64
	Errors             []string
}

// GCStorage defines the database operations needed for garbage collection.
type GCStorage interface {
	GetAllFilePaths() ([]string, error)
}

// GCMediaStorage defines the filesystem operations needed for garbage collection.
type GCMediaStorage interface {
	WalkFiles() ([]string, error)
	GetFileModTime(filePath string) (time.Time, error)
	DeleteFile(filePath string) error
}

// NewMediaGarbageCollector creates a new garbage collector instance.
// safetyThreshold is the minimum age a file must have before being deleted.
// This prevents deletion of files that were just uploaded but not yet committed to DB.
func NewMediaGarbageCollector(
	storage GCStorage,
	mediaStorage GCMediaStorage,
	safetyThreshold time.Duration,
) *MediaGarbageCollector {
	return &MediaGarbageCollector{
		storage:         storage,
		mediaStorage:    mediaStorage,
		safetyThreshold: safetyThreshold,
	}
}

// StartBackgroundCleanup starts a background goroutine that runs cleanup periodically.
// It follows the same pattern as board_access.StartBackgroundUpdate.
func (gc *MediaGarbageCollector) StartBackgroundCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	log.Printf("Started MediaGarbageCollector background cleanup (interval: %v, safety threshold: %v)",
		interval, gc.safetyThreshold)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := gc.RunCleanup(); err != nil {
					log.Printf("MediaGC: cleanup error: %v", err)
				} else {
					stats := gc.GetLastCleanupStats()
					log.Printf("MediaGC: completed - scanned: %d, orphans: %d, deleted: %d, bytes reclaimed: %d, duration: %dms, errors: %d",
						stats.FilesScanned,
						stats.OrphanedFiles,
						stats.FilesDeleted,
						stats.BytesReclaimed,
						stats.DurationMs,
						len(stats.Errors),
					)
				}
			case <-ctx.Done():
				log.Printf("MediaGC: shutting down gracefully")
				return
			}
		}
	}()
}

// RunCleanup executes a single garbage collection cycle.
// It can be called manually for testing or maintenance.
func (gc *MediaGarbageCollector) RunCleanup() error {
	startTime := time.Now()
	stats := CleanupStats{
		RunAt:  startTime,
		Errors: []string{},
	}

	// Step 1: Get all file paths from the database
	dbPaths, err := gc.storage.GetAllFilePaths()
	if err != nil {
		return err
	}

	// Build a set for O(1) lookup
	dbPathSet := make(map[string]bool, len(dbPaths))
	for _, path := range dbPaths {
		// Normalize path separators for cross-platform compatibility
		normalizedPath := filepath.ToSlash(path)
		dbPathSet[normalizedPath] = true
	}

	// Step 2: Walk the filesystem and find orphans
	fsPaths, err := gc.mediaStorage.WalkFiles()
	if err != nil {
		return err
	}
	stats.FilesScanned = len(fsPaths)

	// Step 3: Identify and delete orphaned files
	for _, fsPath := range fsPaths {
		normalizedPath := filepath.ToSlash(fsPath)

		// Check if file exists in database
		if dbPathSet[normalizedPath] {
			continue // File is referenced in DB, keep it
		}

		// File is orphaned - check if it's old enough to delete
		modTime, err := gc.mediaStorage.GetFileModTime(fsPath)
		if err != nil {
			stats.Errors = append(stats.Errors, "stat error: "+fsPath+": "+err.Error())
			continue
		}

		age := time.Since(modTime)
		if age < gc.safetyThreshold {
			// File is too young, might be mid-upload
			continue
		}

		// File is orphaned and old enough - delete it
		stats.OrphanedFiles++

		if err := gc.mediaStorage.DeleteFile(fsPath); err != nil {
			stats.Errors = append(stats.Errors, "delete error: "+fsPath+": "+err.Error())
		} else {
			stats.FilesDeleted++
			// Note: We don't track bytes reclaimed for now (would need file size from stat)
		}
	}

	stats.DurationMs = time.Since(startTime).Milliseconds()
	gc.lastCleanupStats = stats

	return nil
}

// GetLastCleanupStats returns statistics from the last cleanup run.
// Useful for monitoring and observability.
func (gc *MediaGarbageCollector) GetLastCleanupStats() CleanupStats {
	return gc.lastCleanupStats
}
