package service

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	svcutils "github.com/itchan-dev/itchan/backend/internal/service/utils"
	"github.com/itchan-dev/itchan/backend/internal/storage/fs"
	"github.com/itchan-dev/itchan/backend/internal/utils"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/errors"
)

type MessageService interface {
	Create(creationData domain.MessageCreationData) (domain.MsgId, error)
	Get(board domain.BoardShortName, id domain.MsgId) (domain.Message, error)
	Delete(board domain.BoardShortName, id domain.MsgId) error
}

type Message struct {
	storage      MessageStorage
	validator    MessageValidator
	mediaStorage fs.MediaStorage
	cfg          *config.Public
}

type MessageStorage interface {
	CreateMessage(creationData domain.MessageCreationData) (domain.MsgId, error)
	GetMessage(board domain.BoardShortName, id domain.MsgId) (domain.Message, error)
	DeleteMessage(board domain.BoardShortName, id domain.MsgId) error
	AddAttachments(board domain.BoardShortName, messageID domain.MsgId, attachments domain.Attachments) error
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

	// Always create message metadata first (without files) to get msgId
	creationDataWithoutFiles := creationData
	creationDataWithoutFiles.PendingFiles = nil

	msgID, err := b.storage.CreateMessage(creationDataWithoutFiles)
	if err != nil {
		return 0, err
	}

	// Then handle files if present
	if len(creationData.PendingFiles) > 0 {
		if err := b.saveAndAttachFiles(creationData.Board, msgID, creationData.ThreadId, creationData.PendingFiles); err != nil {
			// Cleanup: delete the message since we failed to save attachments
			b.storage.DeleteMessage(creationData.Board, msgID)
			return 0, err
		}
	}

	return msgID, nil
}

// saveAndAttachFiles saves files to storage and adds them as attachments to a message.
// It handles cleanup of saved files if any step fails.
func (b *Message) saveAndAttachFiles(
	board domain.BoardShortName,
	messageID domain.MsgId,
	threadID domain.ThreadId,
	pendingFiles []*domain.PendingFile,
) error {
	var attachments domain.Attachments
	savedFiles := make([]string, 0) // Track for cleanup on error

	for _, pendingFile := range pendingFiles {
		// Capture original metadata BEFORE sanitization
		originalFilename := pendingFile.Filename
		originalMimeType := pendingFile.MimeType

		// Sanitize file (returns new PendingFile with sanitized data)
		sanitizedFile, err := svcutils.SanitizeFile(pendingFile)
		if err != nil {
			// Cleanup saved files
			for _, p := range savedFiles {
				b.mediaStorage.DeleteFile(p)
			}
			return err
		}

		// Save sanitized file to disk
		filePath, err := b.mediaStorage.SaveFile(
			sanitizedFile.Data,
			string(board),
			fmt.Sprintf("%d", threadID),
			sanitizedFile.Filename,
		)
		if err != nil {
			for _, p := range savedFiles {
				b.mediaStorage.DeleteFile(p)
			}
			return fmt.Errorf("failed to save file: %w", err)
		}
		savedFiles = append(savedFiles, filePath)

		// Generate thumbnail for images
		var thumbnailPath *string
		if strings.HasPrefix(sanitizedFile.MimeType, "image/") {
			img, _, err := image.Decode(sanitizedFile.Data)
			if err == nil {
				thumbnail := utils.GenerateThumbnail(img, 125)
				thumbPath, err := b.mediaStorage.SaveThumbnail(thumbnail, filePath)
				if err == nil {
					thumbnailPath = &thumbPath
					savedFiles = append(savedFiles, thumbPath)
				}
			}
			// Note: We don't fail the upload if thumbnail generation fails
		}

		// Create file metadata with BOTH original and sanitized data
		fileData := &domain.File{
			FileCommonMetadata: sanitizedFile.FileCommonMetadata, // Sanitized metadata
			FilePath:           filePath,
			OriginalFilename:   originalFilename,   // User's uploaded filename
			OriginalMimeType:   &originalMimeType, // Pre-sanitization MIME type
			ThumbnailPath:      thumbnailPath,
		}

		// Create attachment
		attachment := &domain.Attachment{
			Board:     board,
			MessageId: messageID,
			File:      fileData,
		}

		attachments = append(attachments, attachment)
	}

	// Add attachments to DB
	err := b.storage.AddAttachments(board, messageID, attachments)
	if err != nil {
		// Cleanup: delete saved files
		for _, savedPath := range savedFiles {
			b.mediaStorage.DeleteFile(savedPath)
		}
		return fmt.Errorf("failed to save attachments to DB: %w", err)
	}

	return nil
}

func (b *Message) Get(board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
	message, err := b.storage.GetMessage(board, id)
	if err != nil {
		return domain.Message{}, err
	}
	return message, nil
}

func (b *Message) Delete(board domain.BoardShortName, id domain.MsgId) error {
	// First, get the message to find its attachments
	msg, err := b.storage.GetMessage(board, id)
	if err != nil {
		return err
	}

	// Delete the message from storage (DB will cascade delete attachments records)
	err = b.storage.DeleteMessage(board, id)
	if err != nil {
		return err
	}

	// Delete the actual files from filesystem
	for _, attachment := range msg.Attachments {
		if attachment.File != nil {
			// Delete original file
			if err := b.mediaStorage.DeleteFile(attachment.File.FilePath); err != nil {
				// Best effort: log errors but don't fail the operation
			}

			// Delete thumbnail if it exists
			if attachment.File.ThumbnailPath != nil {
				if err := b.mediaStorage.DeleteFile(*attachment.File.ThumbnailPath); err != nil {
					// Best effort: log errors but don't fail the operation
				}
			}
		}
	}

	return nil
}
