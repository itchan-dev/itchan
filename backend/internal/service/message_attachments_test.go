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
	saveFileFunc    func(fileData io.Reader, boardID, threadID, originalFilename string) (string, error)
	readFunc        func(filePath string) (io.ReadCloser, error)
	deleteFileFunc  func(filePath string) error
	deleteThreadFunc func(boardID, threadID string) error
	deleteBoardFunc func(boardID string) error

	mu                  sync.Mutex
	saveFileCalls       []SaveFileCall
	deleteFileCalls     []string
	deleteThreadCalls   []DeleteThreadCall
	deleteBoardCalls    []string
}

type SaveFileCall struct {
	BoardID          string
	ThreadID         string
	OriginalFilename string
	Data             []byte
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

// MockStorageWithAddAttachments extends MockMessageStorage with AddAttachments
type MockStorageWithAddAttachments struct {
	MockMessageStorage
	addAttachmentsFunc  func(board domain.BoardShortName, messageID domain.MsgId, attachments domain.Attachments) error
	addAttachmentsCalls []AddAttachmentsCall
	mu2                 sync.Mutex
}

type AddAttachmentsCall struct {
	Board       domain.BoardShortName
	MessageID   domain.MsgId
	Attachments domain.Attachments
}

func (m *MockStorageWithAddAttachments) AddAttachments(board domain.BoardShortName, messageID domain.MsgId, attachments domain.Attachments) error {
	m.mu2.Lock()
	m.addAttachmentsCalls = append(m.addAttachmentsCalls, AddAttachmentsCall{
		Board:       board,
		MessageID:   messageID,
		Attachments: attachments,
	})
	m.mu2.Unlock()

	if m.addAttachmentsFunc != nil {
		return m.addAttachmentsFunc(board, messageID, attachments)
	}
	return nil
}

// --- Helper Functions ---

func createTestConfig() *config.Public {
	return &config.Public{
		MaxAttachmentsPerMessage: 4,
		MaxAttachmentSizeBytes:   10 * 1024 * 1024, // 10MB
		MaxTotalAttachmentSize:   20 * 1024 * 1024, // 20MB
		AllowedImageMimeTypes:    []string{"image/jpeg", "image/png", "image/gif"},
		AllowedVideoMimeTypes:    []string{"video/mp4", "video/webm"},
	}
}

// --- Tests ---

func TestValidatePendingFiles(t *testing.T) {
	cfg := createTestConfig()
	storage := &MockStorageWithAddAttachments{}
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
		storage := &MockStorageWithAddAttachments{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}

		var createdMessageID domain.MsgId = 42

		// Mock storage to return a message ID
		storage.createMessageFunc = func(creationData domain.MessageCreationData) (domain.MsgId, error) {
			// Should be called without PendingFiles
			assert.Nil(t, creationData.PendingFiles)
			return createdMessageID, nil
		}

		// Mock AddAttachments to succeed
		storage.addAttachmentsFunc = func(board domain.BoardShortName, messageID domain.MsgId, attachments domain.Attachments) error {
			assert.Equal(t, createdMessageID, messageID)
			assert.Len(t, attachments, 2)
			return nil
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

		// Verify SaveFile was called twice
		mediaStorage.mu.Lock()
		assert.Len(t, mediaStorage.saveFileCalls, 2)
		assert.Equal(t, "image.jpg", mediaStorage.saveFileCalls[0].OriginalFilename)
		assert.Equal(t, "video.mp4", mediaStorage.saveFileCalls[1].OriginalFilename)
		mediaStorage.mu.Unlock()

		// Verify AddAttachments was called
		storage.mu2.Lock()
		assert.Len(t, storage.addAttachmentsCalls, 1)
		storage.mu2.Unlock()
	})

	t.Run("cleans up files on storage error", func(t *testing.T) {
		cfg := createTestConfig()
		storage := &MockStorageWithAddAttachments{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}

		var createdMessageID domain.MsgId = 42

		storage.createMessageFunc = func(creationData domain.MessageCreationData) (domain.MsgId, error) {
			return createdMessageID, nil
		}

		// Mock AddAttachments to fail
		addAttachmentsError := errors.New("database error")
		storage.addAttachmentsFunc = func(board domain.BoardShortName, messageID domain.MsgId, attachments domain.Attachments) error {
			return addAttachmentsError
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
		assert.Contains(t, err.Error(), "failed to save attachments to DB")

		// Verify files were cleaned up
		mediaStorage.mu.Lock()
		assert.Len(t, mediaStorage.saveFileCalls, 1, "File should have been saved")
		assert.Len(t, mediaStorage.deleteFileCalls, 1, "File should have been deleted on error")
		mediaStorage.mu.Unlock()

		// Verify message was deleted
		storage.mu.Lock()
		assert.True(t, storage.deleteMessageCalled, "Message should have been deleted on error")
		storage.mu.Unlock()
	})

	t.Run("cleans up files on media save error", func(t *testing.T) {
		cfg := createTestConfig()
		storage := &MockStorageWithAddAttachments{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}

		var createdMessageID domain.MsgId = 42

		storage.createMessageFunc = func(creationData domain.MessageCreationData) (domain.MsgId, error) {
			return createdMessageID, nil
		}

		// First SaveFile succeeds, second fails
		callCount := 0
		saveFileError := errors.New("disk full")
		mediaStorage.saveFileFunc = func(fileData io.Reader, boardID, threadID, originalFilename string) (string, error) {
			callCount++
			if callCount == 1 {
				return "tech/1/file1.jpg", nil
			}
			return "", saveFileError
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
		assert.Contains(t, err.Error(), "failed to save file")

		// Verify first file was cleaned up
		mediaStorage.mu.Lock()
		assert.Len(t, mediaStorage.deleteFileCalls, 1, "First file should have been deleted")
		assert.Equal(t, "tech/1/file1.jpg", mediaStorage.deleteFileCalls[0])
		mediaStorage.mu.Unlock()

		// Verify message was deleted
		storage.mu.Lock()
		assert.True(t, storage.deleteMessageCalled)
		storage.mu.Unlock()
	})

	t.Run("validation error prevents file operations", func(t *testing.T) {
		cfg := createTestConfig()
		storage := &MockStorageWithAddAttachments{}
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
		storage := &MockStorageWithAddAttachments{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}

		// Mock GetMessage to return a message with attachments
		storage.getMessageFunc = func(board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
			return domain.Message{
				MessageMetadata: domain.MessageMetadata{
					Id:    id,
					Board: board,
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

		err := service.Delete("tech", 1)
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
		storage := &MockStorageWithAddAttachments{}
		validator := &MockMessageValidator{}
		mediaStorage := &SharedMockMediaStorage{}

		storage.getMessageFunc = func(board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
			return domain.Message{
				MessageMetadata: domain.MessageMetadata{
					Id:    id,
					Board: board,
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
		err := service.Delete("tech", 1)
		assert.NoError(t, err)

		// Message should still be deleted
		storage.mu.Lock()
		assert.True(t, storage.deleteMessageCalled)
		storage.mu.Unlock()
	})
}
