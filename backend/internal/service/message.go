package service

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	svcutils "github.com/itchan-dev/itchan/backend/internal/service/utils"
	"github.com/itchan-dev/itchan/backend/internal/storage/fs"
	"github.com/itchan-dev/itchan/backend/internal/utils"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/errors"
)

type MessageService interface {
	Create(creationData domain.MessageCreationData) (msgId domain.MsgId, err error)
	Get(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error)
	Delete(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) error
}

type Message struct {
	storage      MessageStorage
	validator    MessageValidator
	mediaStorage fs.MediaStorage
	cfg          *config.Public
}

type MessageStorage interface {
	CreateMessage(creationData domain.MessageCreationData, attachments domain.Attachments) (msgId domain.MsgId, err error)
	GetMessage(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error)
	DeleteMessage(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) error
}

type MessageValidator interface {
	Text(text domain.MsgText) error
	PendingFiles(files []*domain.PendingFile) error
}

func NewMessage(storage MessageStorage, validator MessageValidator, mediaStorage fs.MediaStorage, cfg *config.Public) MessageService {
	return &Message{
		storage:      storage,
		validator:    validator,
		mediaStorage: mediaStorage,
		cfg:          cfg,
	}
}

func (b *Message) Create(creationData domain.MessageCreationData) (domain.MsgId, error) {
	// Determine what content we have
	hasFiles := len(creationData.PendingFiles) > 0
	hasText := len(strings.TrimSpace(string(creationData.Text))) > 0

	// Business rule: must have EITHER text OR files
	if !hasText && !hasFiles {
		return 0, &errors.ErrorWithStatusCode{
			Message:    "message must contain either text or attachments",
			StatusCode: 400,
		}
	}

	// Validate text only if text is provided
	if hasText {
		if err := b.validator.Text(creationData.Text); err != nil {
			return 0, err
		}
	}

	// Validate files only if files are provided
	if hasFiles {
		if err := b.validator.PendingFiles(creationData.PendingFiles); err != nil {
			return 0, err
		}
	}

	var attachments domain.Attachments
	var savedFiles []string

	if len(creationData.PendingFiles) > 0 {
		var err error
		attachments, savedFiles, err = b.processAndSaveFiles(
			creationData.Board,
			creationData.ThreadId,
			creationData.PendingFiles,
		)
		if err != nil {
			return 0, err // No DB pollution if file processing fails
		}
	}

	msgID, err := b.storage.CreateMessage(creationData, attachments)
	if err != nil {
		for _, path := range savedFiles {
			b.mediaStorage.DeleteFile(path)
		}
		return 0, err
	}

	return msgID, nil
}

func (b *Message) processAndSaveFiles(
	board domain.BoardShortName,
	threadID domain.ThreadId,
	pendingFiles []*domain.PendingFile,
) (domain.Attachments, []string, error) {
	var attachments domain.Attachments
	savedFiles := make([]string, 0) // Track for cleanup on error

	for _, pendingFile := range pendingFiles {
		var filePath string
		var sanitizedMetadata domain.FileCommonMetadata
		var thumbnailPath *string

		if pendingFile.IsVideo() {
			// Video: Sanitize to temp file and move
			sanitizedVideo, err := svcutils.SanitizeVideo(pendingFile)
			if err != nil {
				// Cleanup saved files
				for _, p := range savedFiles {
					b.mediaStorage.DeleteFile(p)
				}
				return nil, nil, err
			}

			// Extract thumbnail from temp file before move (while we have full path)
			videoThumbnail, _ := svcutils.ExtractVideoThumbnail(sanitizedVideo.TempFilePath)

			// Move temp file to final destination
			filePath, err = b.mediaStorage.MoveFile(
				sanitizedVideo.TempFilePath,
				string(board),
				fmt.Sprintf("%d", threadID),
				sanitizedVideo.Filename,
			)
			if err != nil {
				// Clean up temp file on error
				os.Remove(sanitizedVideo.TempFilePath)
				// Clean up previously saved files
				for _, p := range savedFiles {
					b.mediaStorage.DeleteFile(p)
				}
				return nil, nil, fmt.Errorf("failed to move video file: %w", err)
			}
			// Track saved file immediately after saving
			savedFiles = append(savedFiles, filePath)
			sanitizedMetadata = sanitizedVideo.FileCommonMetadata

			// Save video thumbnail if extraction succeeded
			if videoThumbnail != nil {
				thumbPath, err := b.mediaStorage.SaveThumbnail(videoThumbnail, filePath)
				if err == nil {
					savedFiles = append(savedFiles, thumbPath)
					thumbnailPath = &thumbPath
				}
				// Note: Don't fail upload if thumbnail save fails
			}

		} else if pendingFile.IsImage() {
			sanitizedImage, err := svcutils.SanitizeImage(pendingFile, b.cfg.MaxDecodedImageSize)
			if err != nil {
				// Cleanup saved files
				for _, p := range savedFiles {
					b.mediaStorage.DeleteFile(p)
				}
				return nil, nil, err
			}

			var imageSize int64
			filePath, imageSize, err = b.mediaStorage.SaveImage(
				sanitizedImage.Image.(image.Image),
				sanitizedImage.Format,
				string(board),
				fmt.Sprintf("%d", threadID),
				sanitizedImage.Filename,
			)
			if err != nil {
				for _, p := range savedFiles {
					b.mediaStorage.DeleteFile(p)
				}
				return nil, nil, fmt.Errorf("failed to save image file: %w", err)
			}
			// Track saved file immediately after saving
			savedFiles = append(savedFiles, filePath)
			sanitizedMetadata = sanitizedImage.FileCommonMetadata
			// Update size with actual encoded size
			sanitizedMetadata.SizeBytes = imageSize

			// Generate thumbnail from the SAME decoded image (no re-decode!)
			thumbnail := utils.GenerateThumbnail(sanitizedImage.Image.(image.Image), 150)
			thumbPath, err := b.mediaStorage.SaveThumbnail(thumbnail, filePath)
			if err == nil {
				// Track thumbnail file
				savedFiles = append(savedFiles, thumbPath)
				thumbnailPath = &thumbPath
			}
			// Note: We don't fail the upload if thumbnail generation fails

		} else {
			// Unsupported file type (should not happen if validation is correct)
			return nil, nil, fmt.Errorf("unsupported file type: %s", pendingFile.MimeType)
		}

		// Create file metadata ONCE with both original and sanitized data
		fileData := &domain.File{
			FileCommonMetadata: sanitizedMetadata,
			FilePath:           filePath,
			OriginalFilename:   pendingFile.Filename,
			OriginalMimeType:   pendingFile.MimeType,
			ThumbnailPath:      thumbnailPath,
		}

		// Create attachment (MessageId will be set by storage layer)
		attachment := &domain.Attachment{
			Board: board,
			File:  fileData,
		}

		attachments = append(attachments, attachment)
	}

	return attachments, savedFiles, nil
}

func (b *Message) Get(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) (domain.Message, error) {
	message, err := b.storage.GetMessage(board, threadId, id)
	if err != nil {
		return domain.Message{}, err
	}
	return message, nil
}

func (b *Message) Delete(board domain.BoardShortName, threadId domain.ThreadId, id domain.MsgId) error {
	// First, get the message to find its attachments
	msg, err := b.storage.GetMessage(board, threadId, id)
	if err != nil {
		return err
	}

	// Delete the message from storage (DB will cascade delete attachments records)
	err = b.storage.DeleteMessage(board, threadId, id)
	if err != nil {
		return err
	}

	for _, attachment := range msg.Attachments {
		if attachment.File != nil {
			if err := b.mediaStorage.DeleteFile(attachment.File.FilePath); err != nil {
				// Best effort: log errors but don't fail the operation
			}

			if attachment.File.ThumbnailPath != nil {
				if err := b.mediaStorage.DeleteFile(*attachment.File.ThumbnailPath); err != nil {
					// Best effort: log errors but don't fail the operation
				}
			}
		}
	}

	return nil
}
