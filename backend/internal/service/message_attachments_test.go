package service

import (
	"bytes"
	"errors"
	"image"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Helpers ---

// loadTestImage loads the test image file for use in tests
func loadTestImage(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../test_data/test_img.jpg")
	if err != nil {
		t.Fatalf("Failed to load test image: %v", err)
	}
	return data
}

// loadTestVideo loads the test video file for use in tests
func loadTestVideo(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../test_data/test_video.webm")
	if err != nil {
		t.Fatalf("Failed to load test video: %v", err)
	}
	return data
}

// --- Mocks for Attachment Tests ---

// SharedMockMediaStorage mocks the MediaStorage interface for use across different test files
type SharedMockMediaStorage struct {
	saveFileFunc     func(fileData io.Reader, boardID, threadID, originalFilename string) (string, error)
	saveImageFunc    func(img image.Image, format, boardID, threadID, originalFilename string) (string, int64, error)
	moveFileFunc     func(sourcePath, boardID, threadID, filename string) (string, error)
	readFunc         func(filePath string) (io.ReadCloser, error)
	deleteFileFunc   func(filePath string) error
	deleteThreadFunc func(boardID, threadID string) error
	deleteBoardFunc  func(boardID string) error

	mu                sync.Mutex
	saveFileCalls     []SaveFileCall
	saveImageCalls    []SaveImageCall
	moveFileCalls     []MoveFileCall
	deleteFileCalls   []string
	deleteThreadCalls []DeleteThreadCall
	deleteBoardCalls  []string
}

type SaveFileCall struct {
	BoardID          string
	ThreadID         string
	OriginalFilename string
	Data             []byte
}

type SaveImageCall struct {
	BoardID          string
	ThreadID         string
	OriginalFilename string
	Format           string
}

type MoveFileCall struct {
	SourcePath string
	BoardID    string
	ThreadID   string
	Filename   string
}

type DeleteThreadCall struct {
	BoardID  string
	ThreadID string
}

func (m *SharedMockMediaStorage) SaveFile(fileData io.Reader, boardID, threadID, originalFilename string) (string, error) {
	m.mu.Lock()
	// Read data for tracking
	data, _ := io.ReadAll(fileData)
	m.saveFileCalls = append(m.saveFileCalls, SaveFileCall{
		BoardID:          boardID,
		ThreadID:         threadID,
		OriginalFilename: originalFilename,
		Data:             data,
	})
	m.mu.Unlock()

	if m.saveFileFunc != nil {
		// Reset reader for the function
		return m.saveFileFunc(bytes.NewReader(data), boardID, threadID, originalFilename)
	}
	// Default: return a fake path
	return boardID + "/" + threadID + "/" + originalFilename, nil
}

func (m *SharedMockMediaStorage) SaveImage(img image.Image, format, boardID, threadID, originalFilename string) (string, int64, error) {
	m.mu.Lock()
	m.saveImageCalls = append(m.saveImageCalls, SaveImageCall{
		BoardID:          boardID,
		ThreadID:         threadID,
		OriginalFilename: originalFilename,
		Format:           format,
	})
	m.mu.Unlock()

	if m.saveImageFunc != nil {
		return m.saveImageFunc(img, format, boardID, threadID, originalFilename)
	}
	// Default: return a fake path and size
	return boardID + "/" + threadID + "/" + originalFilename, 1024, nil
}

func (m *SharedMockMediaStorage) MoveFile(sourcePath, boardID, threadID, filename string) (string, error) {
	m.mu.Lock()
	m.moveFileCalls = append(m.moveFileCalls, MoveFileCall{
		SourcePath: sourcePath,
		BoardID:    boardID,
		ThreadID:   threadID,
		Filename:   filename,
	})
	m.mu.Unlock()

	if m.moveFileFunc != nil {
		return m.moveFileFunc(sourcePath, boardID, threadID, filename)
	}
	// Default: return a fake path (simulating successful move)
	return boardID + "/" + threadID + "/" + filename, nil
}

func (m *SharedMockMediaStorage) SaveThumbnail(thumbnail image.Image, originalRelativePath string) (string, error) {
	// Mock implementation - just return a thumbnail path
	return "thumb_" + originalRelativePath, nil
}

func (m *SharedMockMediaStorage) Read(filePath string) (io.ReadCloser, error) {
	if m.readFunc != nil {
		return m.readFunc(filePath)
	}
	return nil, errors.New("not implemented")
}

func (m *SharedMockMediaStorage) DeleteFile(filePath string) error {
	m.mu.Lock()
	m.deleteFileCalls = append(m.deleteFileCalls, filePath)
	m.mu.Unlock()

	if m.deleteFileFunc != nil {
		return m.deleteFileFunc(filePath)
	}
	return nil
}

func (m *SharedMockMediaStorage) DeleteThread(boardID, threadID string) error {
	m.mu.Lock()
	m.deleteThreadCalls = append(m.deleteThreadCalls, DeleteThreadCall{boardID, threadID})
	m.mu.Unlock()

	if m.deleteThreadFunc != nil {
		return m.deleteThreadFunc(boardID, threadID)
	}
	return nil
}

func (m *SharedMockMediaStorage) DeleteBoard(boardID string) error {
	m.mu.Lock()
	m.deleteBoardCalls = append(m.deleteBoardCalls, boardID)
	m.mu.Unlock()

	if m.deleteBoardFunc != nil {
		return m.deleteBoardFunc(boardID)
	}
	return nil
}

// --- Helper Functions ---

func createTestConfig() *config.Public {
	return &config.Public{
		MaxAttachmentsPerMessage: 4,
		MaxAttachmentSizeBytes:   10 * 1024 * 1024, // 10MB
		MaxTotalAttachmentSize:   20 * 1024 * 1024, // 20MB
		MaxDecodedImageSize:      20 * 1024 * 1024, // 20MB decoded pixel buffer
		AllowedImageMimeTypes:    []string{"image/jpeg", "image/png", "image/gif"},
		AllowedVideoMimeTypes:    []string{"video/mp4", "video/webm"},
	}
}

// --- Tests ---

func TestValidatePendingFiles(t *testing.T) {
	cfg := createTestConfig()
	storage := &MockMessageStorage{}
	validator := &MockMessageValidator{}
	mediaStorage := &SharedMockMediaStorage{}

	service := NewMessage(storage, validator, mediaStorage, cfg)

	t.Run("valid files pass validation", func(t *testing.T) {
		validator.pendingFilesFunc = func(files []*domain.PendingFile) error {
			// In real code, this would check the files
			return nil
		}

		creationData := domain.MessageCreationData{
			Board:    "tech",
			ThreadId: 1,
			Author:   domain.User{Id: 1},
			Text:     "Test",
			PendingFiles: []*domain.PendingFile{
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "image.jpg",
						SizeBytes: 1024,
						MimeType:  "image/jpeg",
					},
					Data: bytes.NewReader(loadTestImage(t)),
				},
			},
		}

		_, err := service.Create(creationData)
		assert.NoError(t, err)
	})

	t.Run("rejects too many attachments", func(t *testing.T) {
		validator.pendingFilesFunc = func(files []*domain.PendingFile) error {
			if len(files) > 4 {
				return errors.New("too many attachments: max 4 allowed")
			}
			return nil
		}

		files := make([]*domain.PendingFile, 5) // More than max of 4
		for i := range files {
			files[i] = &domain.PendingFile{
				FileCommonMetadata: domain.FileCommonMetadata{
					Filename:  "image.jpg",
					SizeBytes: 1024,
					MimeType:  "image/jpeg",
				},
				Data: bytes.NewReader(loadTestImage(t)),
			}
		}

		creationData := domain.MessageCreationData{
			Board:        "tech",
			ThreadId:     1,
			Author:       domain.User{Id: 1},
			Text:         "Test",
			PendingFiles: files,
		}

		_, err := service.Create(creationData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too many attachments")
	})

	t.Run("rejects unsupported mime type", func(t *testing.T) {
		validator.pendingFilesFunc = func(files []*domain.PendingFile) error {
			if files[0].MimeType == "application/pdf" {
				return errors.New("unsupported file type: application/pdf")
			}
			return nil
		}

		creationData := domain.MessageCreationData{
			Board:    "tech",
			ThreadId: 1,
			Author:   domain.User{Id: 1},
			Text:     "Test",
			PendingFiles: []*domain.PendingFile{
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "document.pdf",
						SizeBytes: 1024,
						MimeType:  "application/pdf",
					},
					Data: bytes.NewReader(loadTestImage(t)),
				},
			},
		}

		_, err := service.Create(creationData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported file type")
	})

	t.Run("rejects file too large", func(t *testing.T) {
		validator.pendingFilesFunc = func(files []*domain.PendingFile) error {
			if files[0].SizeBytes > 10*1024*1024 {
				return errors.New("file too large: max 10485760 bytes allowed")
			}
			return nil
		}

		creationData := domain.MessageCreationData{
			Board:    "tech",
			ThreadId: 1,
			Author:   domain.User{Id: 1},
			Text:     "Test",
			PendingFiles: []*domain.PendingFile{
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "large.jpg",
						SizeBytes: 11 * 1024 * 1024,
						MimeType:  "image/jpeg",
					},
					Data: bytes.NewReader(make([]byte, 11*1024*1024)), // 11MB
				},
			},
		}

		_, err := service.Create(creationData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file too large")
	})

	t.Run("rejects total size too large", func(t *testing.T) {
		validator.pendingFilesFunc = func(files []*domain.PendingFile) error {
			var totalSize int64
			for _, file := range files {
				totalSize += file.SizeBytes
			}
			if totalSize > 20*1024*1024 {
				return errors.New("total attachments size too large: max 20971520 bytes allowed")
			}
			return nil
		}

		creationData := domain.MessageCreationData{
			Board:    "tech",
			ThreadId: 1,
			Author:   domain.User{Id: 1},
			Text:     "Test",
			PendingFiles: []*domain.PendingFile{
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "file1.jpg",
						SizeBytes: 8 * 1024 * 1024,
						MimeType:  "image/jpeg",
					},
					Data: bytes.NewReader(make([]byte, 8*1024*1024)),
				},
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "file2.jpg",
						SizeBytes: 8 * 1024 * 1024,
						MimeType:  "image/jpeg",
					},
					Data: bytes.NewReader(make([]byte, 8*1024*1024)),
				},
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "file3.jpg",
						SizeBytes: 8 * 1024 * 1024,
						MimeType:  "image/jpeg",
					},
					Data: bytes.NewReader(make([]byte, 8*1024*1024)),
				},
			},
		}

		_, err := service.Create(creationData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "total attachments size too large")
	})

	t.Run("accepts all supported image types", func(t *testing.T) {
		mimeTypes := []string{"image/jpeg", "image/png", "image/gif"}

		for _, mimeType := range mimeTypes {
			validator.pendingFilesFunc = func(files []*domain.PendingFile) error {
				return nil // Accept all
			}

			creationData := domain.MessageCreationData{
				Board:    "tech",
				ThreadId: 1,
				Author:   domain.User{Id: 1},
				Text:     "Test",
				PendingFiles: []*domain.PendingFile{
					{
						FileCommonMetadata: domain.FileCommonMetadata{
							Filename:  "image." + mimeType[6:],
							SizeBytes: 1024,
							MimeType:  mimeType,
						},
						Data: bytes.NewReader(loadTestImage(t)),
					},
				},
			}

			_, err := service.Create(creationData)
			assert.NoError(t, err, "Should accept "+mimeType)
		}
	})

	t.Run("accepts all supported video types", func(t *testing.T) {
		// Only testing webm as we have a test video file for it
		mimeType := "video/webm"
		videoData := loadTestVideo(t)

		validator.pendingFilesFunc = func(files []*domain.PendingFile) error {
			return nil // Accept all
		}

		creationData := domain.MessageCreationData{
			Board:    "tech",
			ThreadId: 1,
			Author:   domain.User{Id: 1},
			Text:     "Test",
			PendingFiles: []*domain.PendingFile{
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "video.webm",
						SizeBytes: int64(len(videoData)),
						MimeType:  mimeType,
					},
					Data: bytes.NewReader(videoData),
				},
			},
		}

		_, err := service.Create(creationData)
		assert.NoError(t, err, "Should accept "+mimeType)
	})
}

func TestCreateMessageWithFiles(t *testing.T) {
	t.Run("successfully creates message with files", func(t *testing.T) {
		cfg := createTestConfig()
		storage := &MockMessageStorage{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}

		var createdMessageID domain.MsgId = 42

		// Mock storage to accept message with attachments
		storage.createMessageFunc = func(creationData domain.MessageCreationData, attachments domain.Attachments) (domain.MsgId, error) {
			// Verify attachments were passed (files processed before DB call)
			assert.Len(t, attachments, 2, "Should have 2 attachments")
			return createdMessageID, nil
		}

		service := NewMessage(storage, validator, mediaStorage, cfg)

		fileData1 := loadTestImage(t)
		fileData2 := loadTestImage(t) // Using JPEG for video test (sanitization not tested here)

		creationData := domain.MessageCreationData{
			Board:    "tech",
			ThreadId: 1,
			Author:   domain.User{Id: 1},
			Text:     "Test message",
			PendingFiles: []*domain.PendingFile{
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "image.jpg",
						SizeBytes: int64(len(fileData1)),
						MimeType:  "image/jpeg",
					},
					Data: bytes.NewReader(fileData1),
				},
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "video.mp4",
						SizeBytes: int64(len(fileData2)),
						MimeType:  "video/mp4",
					},
					Data: bytes.NewReader(fileData2),
				},
			},
		}

		msgID, err := service.Create(creationData)

		require.NoError(t, err)
		assert.Equal(t, createdMessageID, msgID)

		// Verify SaveImage was called once for image, MoveFile once for video
		mediaStorage.mu.Lock()
		assert.Len(t, mediaStorage.saveImageCalls, 1, "Image should use SaveImage")
		assert.Equal(t, "image.jpg", mediaStorage.saveImageCalls[0].OriginalFilename)
		assert.Len(t, mediaStorage.moveFileCalls, 1, "Video should use MoveFile")
		assert.Equal(t, "video.mp4", mediaStorage.moveFileCalls[0].Filename)
		mediaStorage.mu.Unlock()

		// Verify CreateMessage was called with attachments
		storage.mu.Lock()
		assert.True(t, storage.createMessageCalled)
		assert.Len(t, storage.createMessageAttachments, 2)
		storage.mu.Unlock()
	})

	t.Run("cleans up files on storage error", func(t *testing.T) {
		cfg := createTestConfig()
		storage := &MockMessageStorage{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}

		// Mock CreateMessage to fail (simulating DB error during transaction)
		createMessageError := errors.New("database error")
		storage.createMessageFunc = func(creationData domain.MessageCreationData, attachments domain.Attachments) (domain.MsgId, error) {
			return 0, createMessageError
		}

		service := NewMessage(storage, validator, mediaStorage, cfg)

		creationData := domain.MessageCreationData{
			Board:    "tech",
			ThreadId: 1,
			Author:   domain.User{Id: 1},
			Text:     "Test message",
			PendingFiles: []*domain.PendingFile{
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "image.jpg",
						SizeBytes: 4,
						MimeType:  "image/jpeg",
					},
					Data: bytes.NewReader(loadTestImage(t)),
				},
			},
		}

		_, err := service.Create(creationData)

		require.Error(t, err)
		assert.True(t, errors.Is(err, createMessageError), "Should return the CreateMessage error")

		// Verify files were cleaned up (image + thumbnail) after DB error
		mediaStorage.mu.Lock()
		assert.Len(t, mediaStorage.saveImageCalls, 1, "File should have been saved before DB call")
		assert.Len(t, mediaStorage.deleteFileCalls, 2, "Image and thumbnail should have been deleted on error")
		mediaStorage.mu.Unlock()

		// Verify DeleteMessage was NOT called (message was never created in DB)
		storage.mu.Lock()
		assert.False(t, storage.deleteMessageCalled, "DeleteMessage should not be called - message was never created")
		storage.mu.Unlock()
	})

	t.Run("cleans up files on media save error", func(t *testing.T) {
		cfg := createTestConfig()
		storage := &MockMessageStorage{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}

		// CreateMessage should never be called (file processing fails first)
		storage.createMessageFunc = func(creationData domain.MessageCreationData, attachments domain.Attachments) (domain.MsgId, error) {
			t.Fatal("CreateMessage should not be called when file processing fails")
			return 0, nil
		}

		// First SaveImage succeeds, second fails
		callCount := 0
		saveImageError := errors.New("disk full")
		mediaStorage.saveImageFunc = func(img image.Image, format, boardID, threadID, originalFilename string) (string, int64, error) {
			callCount++
			if callCount == 1 {
				return "tech/1/file1.jpg", 1024, nil
			}
			return "", 0, saveImageError
		}

		service := NewMessage(storage, validator, mediaStorage, cfg)

		creationData := domain.MessageCreationData{
			Board:    "tech",
			ThreadId: 1,
			Author:   domain.User{Id: 1},
			Text:     "Test message",
			PendingFiles: []*domain.PendingFile{
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "image1.jpg",
						SizeBytes: 5,
						MimeType:  "image/jpeg",
					},
					Data: bytes.NewReader(loadTestImage(t)),
				},
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "image2.jpg",
						SizeBytes: 5,
						MimeType:  "image/jpeg",
					},
					Data: bytes.NewReader(loadTestImage(t)),
				},
			},
		}

		_, err := service.Create(creationData)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to save image file")

		// Verify first file was cleaned up (image + thumbnail)
		mediaStorage.mu.Lock()
		assert.Len(t, mediaStorage.deleteFileCalls, 2, "First image and thumbnail should have been deleted")
		assert.Contains(t, mediaStorage.deleteFileCalls, "tech/1/file1.jpg")
		mediaStorage.mu.Unlock()

		// Verify CreateMessage was never called (file processing failed before DB)
		storage.mu.Lock()
		assert.False(t, storage.createMessageCalled, "CreateMessage should not be called when file processing fails")
		assert.False(t, storage.deleteMessageCalled, "DeleteMessage should not be called - message was never created")
		storage.mu.Unlock()
	})

	t.Run("validation error prevents file operations", func(t *testing.T) {
		cfg := createTestConfig()
		storage := &MockMessageStorage{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}

		// Mock validator to reject the file
		validator.pendingFilesFunc = func(files []*domain.PendingFile) error {
			return errors.New("file too large: max 10485760 bytes allowed")
		}

		service := NewMessage(storage, validator, mediaStorage, cfg)

		creationData := domain.MessageCreationData{
			Board:    "tech",
			ThreadId: 1,
			Author:   domain.User{Id: 1},
			Text:     "Test message",
			PendingFiles: []*domain.PendingFile{
				{
					FileCommonMetadata: domain.FileCommonMetadata{
						Filename:  "too-large.jpg",
						SizeBytes: 100 * 1024 * 1024, // 100MB - too large
						MimeType:  "image/jpeg",
					},
					Data: bytes.NewReader(loadTestImage(t)),
				},
			},
		}

		_, err := service.Create(creationData)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "file too large")

		// Verify no files were saved
		mediaStorage.mu.Lock()
		assert.Empty(t, mediaStorage.saveFileCalls)
		mediaStorage.mu.Unlock()

		// Verify no message was created
		storage.mu.Lock()
		assert.False(t, storage.createMessageCalled)
		storage.mu.Unlock()
	})
}

func TestMessageDeleteWithAttachments(t *testing.T) {
	t.Run("deletes message and all attachment files", func(t *testing.T) {
		cfg := createTestConfig()
		storage := &MockMessageStorage{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}

		// Mock GetMessage to return a message with attachments
		storage.getMessageFunc = func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error) {
			return domain.Message{
				MessageMetadata: domain.MessageMetadata{
					Id:       id,
					ThreadId: threadId,
					Board:    board,
				},
				Attachments: domain.Attachments{
					&domain.Attachment{
						File: &domain.File{
							FilePath: "tech/1/file1.jpg",
						},
					},
					&domain.Attachment{
						File: &domain.File{
							FilePath: "tech/1/file2.png",
						},
					},
				},
			}, nil
		}

		service := NewMessage(storage, validator, mediaStorage, cfg)

		err := service.Delete("tech", 1, 1)
		require.NoError(t, err)

		// Verify message was deleted
		storage.mu.Lock()
		assert.True(t, storage.deleteMessageCalled)
		assert.Equal(t, domain.MsgId(1), storage.deleteMessageArgId)
		storage.mu.Unlock()

		// Verify both files were deleted
		mediaStorage.mu.Lock()
		assert.Len(t, mediaStorage.deleteFileCalls, 2)
		assert.Contains(t, mediaStorage.deleteFileCalls, "tech/1/file1.jpg")
		assert.Contains(t, mediaStorage.deleteFileCalls, "tech/1/file2.png")
		mediaStorage.mu.Unlock()
	})

	t.Run("continues despite file deletion errors", func(t *testing.T) {
		cfg := createTestConfig()
		storage := &MockMessageStorage{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}

		storage.getMessageFunc = func(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error) {
			return domain.Message{
				MessageMetadata: domain.MessageMetadata{
					Id:       id,
					ThreadId: threadId,
					Board:    board,
				},
				Attachments: domain.Attachments{
					&domain.Attachment{
						File: &domain.File{
							FilePath: "tech/1/file1.jpg",
						},
					},
				},
			}, nil
		}

		// DeleteFile fails
		mediaStorage.deleteFileFunc = func(filePath string) error {
			return errors.New("file not found")
		}

		service := NewMessage(storage, validator, mediaStorage, cfg)

		// Should not error despite file deletion failure
		err := service.Delete("tech", 1, 1)
		assert.NoError(t, err)

		// Message should still be deleted
		storage.mu.Lock()
		assert.True(t, storage.deleteMessageCalled)
		storage.mu.Unlock()
	})
}
