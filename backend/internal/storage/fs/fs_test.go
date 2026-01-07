package fs

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew tests the Storage constructor
func TestNew(t *testing.T) {
	t.Run("creates storage with valid path", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage, err := New(tmpDir)

		require.NoError(t, err)
		assert.NotNil(t, storage)
		assert.Equal(t, tmpDir, storage.rootPath)

		// Verify directory exists
		_, err = os.Stat(tmpDir)
		assert.NoError(t, err)
	})

	t.Run("creates nested directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		nestedPath := filepath.Join(tmpDir, "a", "b", "c")

		storage, err := New(nestedPath)

		require.NoError(t, err)
		assert.NotNil(t, storage)

		// Verify nested directory exists
		_, err = os.Stat(nestedPath)
		assert.NoError(t, err)
	})

	t.Run("cleans path to prevent traversal", func(t *testing.T) {
		tmpDir := t.TempDir()
		dirtyPath := filepath.Join(tmpDir, "media", "..", "media")

		storage, err := New(dirtyPath)

		require.NoError(t, err)
		// Path should be cleaned
		assert.Equal(t, filepath.Join(tmpDir, "media"), storage.rootPath)
	})
}

// TestSaveFile tests the SaveFile method
func TestSaveFile(t *testing.T) {
	t.Run("saves file successfully", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		content := []byte("test file content")
		reader := bytes.NewReader(content)

		path, err := storage.SaveFile(reader, "board1", "thread1", "image.jpg")

		require.NoError(t, err)
		assert.NotEmpty(t, path)

		// Verify path structure
		assert.Contains(t, path, "board1")
		assert.Contains(t, path, "thread1")
		assert.Contains(t, path, ".jpg")

		// Verify file exists and has correct content
		fullPath := filepath.Join(storage.rootPath, path)
		savedContent, err := os.ReadFile(fullPath)
		require.NoError(t, err)
		assert.Equal(t, content, savedContent)
	})

	t.Run("generates unique filenames", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		content := []byte("test")

		// Save same file twice
		path1, err := storage.SaveFile(bytes.NewReader(content), "board1", "thread1", "image.jpg")
		require.NoError(t, err)

		path2, err := storage.SaveFile(bytes.NewReader(content), "board1", "thread1", "image.jpg")
		require.NoError(t, err)

		// Paths should be different
		assert.NotEqual(t, path1, path2)

		// Both files should exist
		_, err = os.Stat(filepath.Join(storage.rootPath, path1))
		assert.NoError(t, err)
		_, err = os.Stat(filepath.Join(storage.rootPath, path2))
		assert.NoError(t, err)
	})

	t.Run("preserves file extension", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		testCases := []struct {
			filename string
			ext      string
		}{
			{"image.jpg", ".jpg"},
			{"video.mp4", ".mp4"},
			{"document.pdf", ".pdf"},
			{"file.tar.gz", ".gz"},
		}

		for _, tc := range testCases {
			t.Run(tc.filename, func(t *testing.T) {
				path, err := storage.SaveFile(bytes.NewReader([]byte("test")), "b", "t", tc.filename)
				require.NoError(t, err)
				assert.True(t, strings.HasSuffix(path, tc.ext))
			})
		}
	})

	t.Run("handles files without extension", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		path, err := storage.SaveFile(bytes.NewReader([]byte("test")), "b", "t", "noextension")
		require.NoError(t, err)
		assert.NotEmpty(t, path)
	})

	t.Run("creates board and thread directories", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		path, err := storage.SaveFile(bytes.NewReader([]byte("test")), "newboard", "newthread", "file.txt")
		require.NoError(t, err)

		// Verify directory structure
		boardDir := filepath.Join(storage.rootPath, "newboard")
		threadDir := filepath.Join(storage.rootPath, "newboard", "newthread")

		_, err = os.Stat(boardDir)
		assert.NoError(t, err)
		_, err = os.Stat(threadDir)
		assert.NoError(t, err)

		// Verify file is in correct location
		assert.True(t, strings.HasPrefix(path, "newboard"+string(filepath.Separator)+"newthread"))
	})

	t.Run("handles empty reader", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		path, err := storage.SaveFile(bytes.NewReader([]byte{}), "b", "t", "empty.txt")
		require.NoError(t, err)

		// File should exist but be empty
		fullPath := filepath.Join(storage.rootPath, path)
		content, err := os.ReadFile(fullPath)
		require.NoError(t, err)
		assert.Empty(t, content)
	})

	t.Run("sanitizes filename", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		// Test with potentially problematic filename
		path, err := storage.SaveFile(bytes.NewReader([]byte("test")), "b", "t", "../../etc/passwd.jpg")
		require.NoError(t, err)

		// Path should not contain traversal
		assert.NotContains(t, path, "..")
		assert.Contains(t, path, ".jpg")
	})
}

// TestMoveFile tests the MoveFile method
func TestMoveFile(t *testing.T) {
	t.Run("moves file successfully", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		// Create a temp file to move
		tmpFile, err := os.CreateTemp("", "test_video_*.mp4")
		require.NoError(t, err)
		tmpPath := tmpFile.Name()
		content := []byte("video content")
		_, err = tmpFile.Write(content)
		require.NoError(t, err)
		tmpFile.Close()

		// Move the file
		path, err := storage.MoveFile(tmpPath, "board1", "thread1", "video.mp4")
		require.NoError(t, err)
		assert.NotEmpty(t, path)

		// Verify path structure
		assert.Contains(t, path, "board1")
		assert.Contains(t, path, "thread1")
		assert.Contains(t, path, ".mp4")

		// Verify file exists at destination
		fullPath := filepath.Join(storage.rootPath, path)
		movedContent, err := os.ReadFile(fullPath)
		require.NoError(t, err)
		assert.Equal(t, content, movedContent)

		// Verify source file is deleted
		_, err = os.Stat(tmpPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("generates unique filenames", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		content := []byte("test content")

		// Create two temp files
		tmpFile1, _ := os.CreateTemp("", "video_*.mp4")
		tmpPath1 := tmpFile1.Name()
		tmpFile1.Write(content)
		tmpFile1.Close()

		tmpFile2, _ := os.CreateTemp("", "video_*.mp4")
		tmpPath2 := tmpFile2.Name()
		tmpFile2.Write(content)
		tmpFile2.Close()

		// Move both files
		path1, err := storage.MoveFile(tmpPath1, "board1", "thread1", "video.mp4")
		require.NoError(t, err)

		path2, err := storage.MoveFile(tmpPath2, "board1", "thread1", "video.mp4")
		require.NoError(t, err)

		// Paths should be different
		assert.NotEqual(t, path1, path2)

		// Both files should exist
		_, err = os.Stat(filepath.Join(storage.rootPath, path1))
		assert.NoError(t, err)
		_, err = os.Stat(filepath.Join(storage.rootPath, path2))
		assert.NoError(t, err)
	})

	t.Run("creates board and thread directories", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		// Create temp file
		tmpFile, _ := os.CreateTemp("", "test_*.txt")
		tmpPath := tmpFile.Name()
		tmpFile.Write([]byte("test"))
		tmpFile.Close()

		path, err := storage.MoveFile(tmpPath, "newboard", "newthread", "file.txt")
		require.NoError(t, err)

		// Verify directory structure
		boardDir := filepath.Join(storage.rootPath, "newboard")
		threadDir := filepath.Join(storage.rootPath, "newboard", "newthread")

		_, err = os.Stat(boardDir)
		assert.NoError(t, err)
		_, err = os.Stat(threadDir)
		assert.NoError(t, err)

		// Verify file is in correct location
		assert.True(t, strings.HasPrefix(path, "newboard"+string(filepath.Separator)+"newthread"))
	})

	t.Run("removes source file after successful move", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		// Create temp file
		tmpFile, _ := os.CreateTemp("", "test_*.mp4")
		tmpPath := tmpFile.Name()
		tmpFile.Write([]byte("content"))
		tmpFile.Close()

		// Verify source exists
		_, err = os.Stat(tmpPath)
		require.NoError(t, err)

		// Move file
		_, err = storage.MoveFile(tmpPath, "b", "t", "video.mp4")
		require.NoError(t, err)

		// Verify source is deleted
		_, err = os.Stat(tmpPath)
		assert.True(t, os.IsNotExist(err), "Source file should be deleted")
	})

	t.Run("returns error if source file doesn't exist", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		_, err = storage.MoveFile("/nonexistent/file.mp4", "b", "t", "video.mp4")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open source file")
	})

	t.Run("preserves file extension", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		testCases := []struct {
			filename string
			ext      string
		}{
			{"video.mp4", ".mp4"},
			{"video.webm", ".webm"},
			{"file.tar.gz", ".gz"},
		}

		for _, tc := range testCases {
			t.Run(tc.filename, func(t *testing.T) {
				tmpFile, _ := os.CreateTemp("", "test_*")
				tmpPath := tmpFile.Name()
				tmpFile.Write([]byte("test"))
				tmpFile.Close()

				path, err := storage.MoveFile(tmpPath, "b", "t", tc.filename)
				require.NoError(t, err)
				assert.True(t, strings.HasSuffix(path, tc.ext))
			})
		}
	})
}

// TestRead tests the Read method
func TestRead(t *testing.T) {
	t.Run("reads existing file", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		// Save a file first
		content := []byte("test content")
		path, err := storage.SaveFile(bytes.NewReader(content), "b", "t", "file.txt")
		require.NoError(t, err)

		// Read it back
		reader, err := storage.Read(path)
		require.NoError(t, err)
		defer reader.Close()

		readContent, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, content, readContent)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		_, err = storage.Read("nonexistent/path/file.txt")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "attachment not found")
	})

	t.Run("handles path traversal attempts", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		// Try to read outside root
		_, err = storage.Read("../../etc/passwd")
		assert.Error(t, err)
	})
}

// TestDeleteFile tests the DeleteFile method
func TestDeleteFile(t *testing.T) {
	t.Run("deletes existing file", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		// Create a file
		path, err := storage.SaveFile(bytes.NewReader([]byte("test")), "b", "t", "file.txt")
		require.NoError(t, err)

		// Verify it exists
		fullPath := filepath.Join(storage.rootPath, path)
		_, err = os.Stat(fullPath)
		require.NoError(t, err)

		// Delete it
		err = storage.DeleteFile(path)
		require.NoError(t, err)

		// Verify it's gone
		_, err = os.Stat(fullPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("succeeds when file doesn't exist", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		// Try to delete non-existent file
		err = storage.DeleteFile("b/t/nonexistent.txt")
		assert.NoError(t, err) // Should not error
	})

	t.Run("deletes multiple files independently", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		// Create multiple files
		path1, _ := storage.SaveFile(bytes.NewReader([]byte("1")), "b", "t", "file1.txt")
		path2, _ := storage.SaveFile(bytes.NewReader([]byte("2")), "b", "t", "file2.txt")

		// Delete first file
		err = storage.DeleteFile(path1)
		require.NoError(t, err)

		// Second file should still exist
		reader, err := storage.Read(path2)
		assert.NoError(t, err)
		if reader != nil {
			reader.Close()
		}
	})
}

// TestDeleteThread tests the DeleteThread method
func TestDeleteThread(t *testing.T) {
	t.Run("deletes thread directory and all files", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		// Create multiple files in the thread
		_, err = storage.SaveFile(bytes.NewReader([]byte("1")), "board1", "thread1", "file1.txt")
		require.NoError(t, err)
		_, err = storage.SaveFile(bytes.NewReader([]byte("2")), "board1", "thread1", "file2.txt")
		require.NoError(t, err)

		threadPath := filepath.Join(storage.rootPath, "board1", "thread1")

		// Verify thread directory exists
		_, err = os.Stat(threadPath)
		require.NoError(t, err)

		// Delete thread
		err = storage.DeleteThread("board1", "thread1")
		require.NoError(t, err)

		// Verify thread directory is gone
		_, err = os.Stat(threadPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("doesn't delete other threads", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		// Create files in two threads
		path1, _ := storage.SaveFile(bytes.NewReader([]byte("1")), "board1", "thread1", "file.txt")
		path2, _ := storage.SaveFile(bytes.NewReader([]byte("2")), "board1", "thread2", "file.txt")

		// Delete thread1
		err = storage.DeleteThread("board1", "thread1")
		require.NoError(t, err)

		// thread2 should still exist
		reader, err := storage.Read(path2)
		assert.NoError(t, err)
		if reader != nil {
			reader.Close()
		}

		// thread1 should be gone
		_, err = storage.Read(path1)
		assert.Error(t, err)
	})

	t.Run("succeeds when thread doesn't exist", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		err = storage.DeleteThread("board1", "nonexistent")
		assert.NoError(t, err)
	})
}

// TestDeleteBoard tests the DeleteBoard method
func TestDeleteBoard(t *testing.T) {
	t.Run("deletes board directory and all threads", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		// Create files in multiple threads
		_, err = storage.SaveFile(bytes.NewReader([]byte("1")), "board1", "thread1", "file1.txt")
		require.NoError(t, err)
		_, err = storage.SaveFile(bytes.NewReader([]byte("2")), "board1", "thread2", "file2.txt")
		require.NoError(t, err)

		boardPath := filepath.Join(storage.rootPath, "board1")

		// Verify board directory exists
		_, err = os.Stat(boardPath)
		require.NoError(t, err)

		// Delete board
		err = storage.DeleteBoard("board1")
		require.NoError(t, err)

		// Verify board directory is gone
		_, err = os.Stat(boardPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("doesn't delete other boards", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		// Create files in two boards
		path1, _ := storage.SaveFile(bytes.NewReader([]byte("1")), "board1", "thread1", "file.txt")
		path2, _ := storage.SaveFile(bytes.NewReader([]byte("2")), "board2", "thread1", "file.txt")

		// Delete board1
		err = storage.DeleteBoard("board1")
		require.NoError(t, err)

		// board2 should still exist
		reader, err := storage.Read(path2)
		assert.NoError(t, err)
		if reader != nil {
			reader.Close()
		}

		// board1 should be gone
		_, err = storage.Read(path1)
		assert.Error(t, err)
	})

	t.Run("succeeds when board doesn't exist", func(t *testing.T) {
		storage, err := New(t.TempDir())
		require.NoError(t, err)

		err = storage.DeleteBoard("nonexistent")
		assert.NoError(t, err)
	})
}

// TestFullWorkflow tests a complete workflow
func TestFullWorkflow(t *testing.T) {
	storage, err := New(t.TempDir())
	require.NoError(t, err)

	// 1. Save files in multiple threads
	file1, err := storage.SaveFile(bytes.NewReader([]byte("content1")), "tech", "1", "image1.jpg")
	require.NoError(t, err)

	file2, err := storage.SaveFile(bytes.NewReader([]byte("content2")), "tech", "1", "image2.png")
	require.NoError(t, err)

	file3, err := storage.SaveFile(bytes.NewReader([]byte("content3")), "tech", "2", "video.mp4")
	require.NoError(t, err)

	// 2. Read files back
	reader1, err := storage.Read(file1)
	require.NoError(t, err)
	content1, _ := io.ReadAll(reader1)
	reader1.Close()
	assert.Equal(t, []byte("content1"), content1)

	// 3. Delete individual file
	err = storage.DeleteFile(file2)
	require.NoError(t, err)

	_, err = storage.Read(file2)
	assert.Error(t, err) // Should be gone

	// 4. file1 should still exist
	reader2, err := storage.Read(file1)
	assert.NoError(t, err)
	reader2.Close()

	// 5. Delete thread
	err = storage.DeleteThread("tech", "1")
	require.NoError(t, err)

	_, err = storage.Read(file1)
	assert.Error(t, err) // Should be gone

	// 6. file3 in different thread should still exist
	reader3, err := storage.Read(file3)
	assert.NoError(t, err)
	reader3.Close()

	// 7. Delete board
	err = storage.DeleteBoard("tech")
	require.NoError(t, err)

	_, err = storage.Read(file3)
	assert.Error(t, err) // Should be gone
}
