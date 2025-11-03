// in storage/localfs/localfs.go
package fs

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/itchan-dev/itchan/backend/internal/service"
)

type Storage struct {
	rootPath string
}

// Ensure Storage struct implements the interface at compile time.
var _ service.MediaStorage = (*Storage)(nil)

func New(rootPath string) (*Storage, error) {
	// Use filepath.Clean to prevent path traversal issues like "media/../"
	p := filepath.Clean(rootPath)

	// Ensure the root directory exists.
	// os.ModePerm (0777) is masked by the system's umask. 0755 is a common, safer default.
	if err := os.MkdirAll(p, 0755); err != nil {
		return nil, fmt.Errorf("failed to create root storage directory %s: %w", p, err)
	}

	return &Storage{rootPath: p}, nil
}

// SaveFile writes a file to the configured storage path with a unique generated filename.
func (s *Storage) SaveFile(fileData io.Reader, boardID, threadID, originalFilename string) (string, error) {
	// Step 1: Generate a unique filename
	// Format: {timestamp}_{random}_{sanitized_original_name}
	ext := filepath.Ext(originalFilename)
	baseName := originalFilename[:len(originalFilename)-len(ext)]

	// Generate random bytes for uniqueness
	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		return "", fmt.Errorf("failed to generate random filename: %w", err)
	}
	randStr := hex.EncodeToString(randBytes)

	// Create unique filename: timestamp_random_originalname.ext
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%d_%s_%s%s", timestamp, randStr, filepath.Base(baseName), ext)

	// Step 2: Construct the relative and full paths.
	relativePath := filepath.Join(boardID, threadID, filename)
	fullPath := filepath.Join(s.rootPath, relativePath)

	// Step 3: Lazily create the board/thread subdirectories.
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create subdirectories: %w", err)
	}

	// Step 4: Create and write the file.
	dst, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fileData); err != nil {
		// Important: If the copy fails, we should try to clean up the empty file.
		os.Remove(fullPath) // Best effort, ignore error here.
		return "", fmt.Errorf("failed to copy file data: %w", err)
	}

	return relativePath, nil
}

// SaveThumbnail saves a thumbnail image as JPEG in the same directory as the original file.
// The thumbnail filename is prefixed with "thumb_".
// Returns the relative path to the thumbnail.
func (s *Storage) SaveThumbnail(thumbnail image.Image, originalRelativePath string) (string, error) {
	// Generate thumbnail filename by prefixing with "thumb_" and changing extension to .jpg
	dir := filepath.Dir(originalRelativePath)
	originalFilename := filepath.Base(originalRelativePath)
	ext := filepath.Ext(originalFilename)
	baseName := originalFilename[:len(originalFilename)-len(ext)]
	thumbnailFilename := fmt.Sprintf("thumb_%s.jpg", baseName)

	// Construct paths
	thumbnailRelativePath := filepath.Join(dir, thumbnailFilename)
	thumbnailFullPath := filepath.Join(s.rootPath, thumbnailRelativePath)

	// Ensure directory exists (should already exist from SaveFile, but be safe)
	if err := os.MkdirAll(filepath.Dir(thumbnailFullPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// Encode thumbnail as JPEG to a buffer first
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: 85}); err != nil {
		return "", fmt.Errorf("failed to encode thumbnail as JPEG: %w", err)
	}

	// Write thumbnail to file
	dst, err := os.Create(thumbnailFullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create thumbnail file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, &buf); err != nil {
		os.Remove(thumbnailFullPath) // Best effort cleanup
		return "", fmt.Errorf("failed to write thumbnail data: %w", err)
	}

	return thumbnailRelativePath, nil
}

// Read opens a file for reading from the storage.
// It corresponds to your requirement #3: "Read certain file".
func (s *Storage) Read(filePath string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.rootPath, filePath)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return a domain-specific error for not found
			return nil, fmt.Errorf("attachment not found: %w", err)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// DeleteFile removes a single file from storage.
// It corresponds to your requirement #4: "Delete certain file".
func (s *Storage) DeleteFile(filePath string) error {
	fullPath := filepath.Join(s.rootPath, filePath)

	err := os.Remove(fullPath)
	if err != nil && !os.IsNotExist(err) {
		// We don't error if the file is already gone, but we do for other errors.
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// DeleteThread removes an entire thread's directory.
// It corresponds to your requirement #5: "Delete certain thread".
func (s *Storage) DeleteThread(boardID, threadID string) error {
	threadPath := filepath.Join(s.rootPath, boardID, threadID)
	err := os.RemoveAll(threadPath)
	if err != nil {
		return fmt.Errorf("failed to delete thread directory: %w", err)
	}
	return nil
}

// DeleteBoard removes an entire board's directory.
// It corresponds to your requirement #6: "Delete certain board".
func (s *Storage) DeleteBoard(boardID string) error {
	boardPath := filepath.Join(s.rootPath, boardID)
	err := os.RemoveAll(boardPath)
	if err != nil {
		return fmt.Errorf("failed to delete board directory: %w", err)
	}
	return nil
}

// WalkFiles walks the entire storage directory and returns all file paths
// relative to the root. This is used by the garbage collector.
func (s *Storage) WalkFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(s.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// If there's an error accessing this path, log but continue
			return nil
		}

		// Skip directories, only collect files
		if info.IsDir() {
			return nil
		}

		// Get relative path from root
		relPath, err := filepath.Rel(s.rootPath, path)
		if err != nil {
			// If we can't get relative path, skip this file
			return nil
		}

		files = append(files, relPath)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk filesystem: %w", err)
	}

	return files, nil
}

// GetFileModTime returns the modification time of a file.
// Used by garbage collector to implement safety threshold.
func (s *Storage) GetFileModTime(filePath string) (time.Time, error) {
	fullPath := filepath.Join(s.rootPath, filePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to stat file: %w", err)
	}
	return info.ModTime(), nil
}
