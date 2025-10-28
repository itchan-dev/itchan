// in storage/localfs/localfs.go
package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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

// Save writes a file to the configured storage path.
func (s *Storage) Save(fileData io.Reader, boardID, threadID, messageID, originalExtension string) (string, error) {
	// Step 1: Generate a safe, unique filename internally.
	// We use the messageID and the original extension.
	// We also clean the extension to prevent shenanigans like ".jpg/../../foo.txt".
	cleanExtension := filepath.Clean(originalExtension)
	filename := fmt.Sprintf("%s%s", messageID, cleanExtension)

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
