// in storage/localfs/localfs.go
package fs

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MediaStorage interface {
	// SaveFile stores a file's content.
	// It takes the board and thread IDs to construct the path and generates a unique filename.
	// It returns the relative path where the file was stored.
	SaveFile(fileData io.Reader, boardID, threadID, originalFilename string) (string, error)

	// SaveImage encodes and saves an image.Image (PNG if format="png", JPEG otherwise).
	// It returns the relative path where the image was stored.
	SaveImage(img image.Image, format, boardID, threadID, originalFilename string) (string, error)

	// MoveFile moves a file from sourcePath to the storage location.
	// Used for sanitized videos to avoid loading into memory.
	// The source file will be deleted after successful move.
	// Returns the relative path where the file was stored.
	MoveFile(sourcePath, boardID, threadID, filename string) (string, error)

	// SaveThumbnail saves a thumbnail image as JPEG.
	// It returns the relative path where the thumbnail was stored.
	SaveThumbnail(thumbnail image.Image, originalRelativePath string) (string, error)

	// Read opens a file for reading given its relative path.
	Read(filePath string) (io.ReadCloser, error)

	// DeleteFile removes a single file.
	DeleteFile(filePath string) error

	// DeleteThread removes all media for an entire thread.
	DeleteThread(boardID, threadID string) error

	// DeleteBoard removes all media for an entire board.
	DeleteBoard(boardID string) error
}

type Storage struct {
	rootPath string
}

// Ensure Storage struct implements the interface at compile time.
var _ MediaStorage = (*Storage)(nil)

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

// generateUniqueFilename generates a unique filename based on a pattern.
// The pattern works like os.CreateTemp: "*" is replaced with timestamp_random.
// Examples:
//   - "*.jpg"        → "1234567890_abc123def456.jpg"
//   - "thumb_*.jpg"  → "thumb_1234567890_abc123def456.jpg"
//   - "*"            → "1234567890_abc123def456"
func generateUniqueFilename(pattern string) (string, error) {
	// Generate random bytes for uniqueness
	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	randStr := hex.EncodeToString(randBytes)

	// Create unique string: timestamp_random
	timestamp := time.Now().Unix()
	uniqueStr := fmt.Sprintf("%d_%s", timestamp, randStr)

	// Replace * with unique string
	filename := strings.Replace(pattern, "*", uniqueStr, 1)
	return filename, nil
}

// saveFile writes file data to the specified path.
// This is an internal helper that just handles the file I/O.
// The caller is responsible for generating unique filenames and constructing paths.
func (s *Storage) saveFile(fileData io.Reader, relativePath string) error {
	fullPath := filepath.Join(s.rootPath, relativePath)

	// Create subdirectories if needed
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create subdirectories: %w", err)
	}

	// Create and write the file
	dst, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fileData); err != nil {
		// Important: If the copy fails, we should try to clean up the empty file.
		os.Remove(fullPath) // Best effort, ignore error here.
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	return nil
}

// SaveFile writes a file to the configured storage path with a unique generated filename.
// The original filename is only used to extract the extension. The actual filename is generated
// to ensure uniqueness and improve privacy (no user-provided names in storage).
func (s *Storage) SaveFile(fileData io.Reader, boardID, threadID, originalFilename string) (string, error) {
	// Extract extension from original filename
	ext := filepath.Ext(originalFilename)

	// Generate unique filename pattern: *.ext (e.g., "*.jpg")
	pattern := "*" + ext
	filename, err := generateUniqueFilename(pattern)
	if err != nil {
		return "", err
	}

	// Construct relative path
	relativePath := filepath.Join(boardID, threadID, filename)

	// Save file
	if err := s.saveFile(fileData, relativePath); err != nil {
		return "", err
	}

	return relativePath, nil
}

// SaveImage encodes and saves an image.Image.
// PNG format is preserved, all others are encoded as JPEG with quality 85.
// The originalFilename is only used to extract the extension.
func (s *Storage) SaveImage(img image.Image, format, boardID, threadID, originalFilename string) (string, error) {
	// Encode image to buffer
	var buf bytes.Buffer
	var err error
	var ext string

	if format == "png" {
		err = png.Encode(&buf, img)
		ext = ".png"
	} else {
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
		ext = ".jpg"
	}

	if err != nil {
		return "", fmt.Errorf("failed to encode image: %w", err)
	}

	// Generate unique filename pattern: *.ext (e.g., "*.jpg")
	pattern := "*" + ext
	filename, err := generateUniqueFilename(pattern)
	if err != nil {
		return "", err
	}

	// Construct relative path
	relativePath := filepath.Join(boardID, threadID, filename)

	// Save file
	if err := s.saveFile(&buf, relativePath); err != nil {
		return "", err
	}

	return relativePath, nil
}

// MoveFile moves a file from sourcePath to the storage location.
// This is more efficient than SaveFile for large files already on disk.
// The filename parameter is only used to extract the extension.
func (s *Storage) MoveFile(sourcePath, boardID, threadID, filename string) (string, error) {
	// Extract extension from filename
	ext := filepath.Ext(filename)

	// Generate unique filename pattern: *.ext
	pattern := "*" + ext
	uniqueFilename, err := generateUniqueFilename(pattern)
	if err != nil {
		return "", err
	}

	// Construct destination path
	relativePath := filepath.Join(boardID, threadID, uniqueFilename)
	destPath := filepath.Join(s.rootPath, relativePath)

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create subdirectories: %w", err)
	}

	// Move file (try rename first, fall back to copy+delete)
	err = os.Rename(sourcePath, destPath)
	if err != nil {
		// Rename failed (different filesystem?), fall back to copy
		src, err := os.Open(sourcePath)
		if err != nil {
			return "", fmt.Errorf("failed to open source file: %w", err)
		}
		defer src.Close()

		dst, err := os.Create(destPath)
		if err != nil {
			return "", fmt.Errorf("failed to create destination file: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			os.Remove(destPath) // Cleanup partial copy
			return "", fmt.Errorf("failed to copy file: %w", err)
		}

		// Remove source after successful copy
		if err := os.Remove(sourcePath); err != nil {
			// Log warning but don't fail - file was copied successfully
			// In production, this should be logged
		}
	}

	return relativePath, nil
}

// SaveThumbnail saves a thumbnail image as JPEG in the same directory as the original file.
// The thumbnail filename is generated with "thumb_" prefix for easy identification.
// Returns the relative path to the thumbnail.
func (s *Storage) SaveThumbnail(thumbnail image.Image, originalRelativePath string) (string, error) {
	// Get directory from original file path
	dir := filepath.Dir(originalRelativePath)

	// Generate unique thumbnail filename: thumb_*.jpg
	thumbnailFilename, err := generateUniqueFilename("thumb_*.jpg")
	if err != nil {
		return "", err
	}

	// Construct relative path
	thumbnailRelativePath := filepath.Join(dir, thumbnailFilename)

	// Encode thumbnail as JPEG to a buffer
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumbnail, &jpeg.Options{Quality: 75}); err != nil {
		return "", fmt.Errorf("failed to encode thumbnail as JPEG: %w", err)
	}

	// Save file
	if err := s.saveFile(&buf, thumbnailRelativePath); err != nil {
		return "", err
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
