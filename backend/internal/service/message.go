package service

import (
	"fmt"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
)

type MessageService interface {
	Create(creationData domain.MessageCreationData) (domain.MsgId, error)
	Get(board domain.BoardShortName, id domain.MsgId) (domain.Message, error)
	Delete(board domain.BoardShortName, id domain.MsgId) error
}

type Message struct {
	storage      MessageStorage
	validator    MessageValidator
	mediaStorage MediaStorage
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

func NewMessage(storage MessageStorage, validator MessageValidator, mediaStorage MediaStorage, cfg *config.Public) MessageService {
	return &Message{
		storage:      storage,
		validator:    validator,
		mediaStorage: mediaStorage,
		cfg:          cfg,
	}
}

func (b *Message) Create(creationData domain.MessageCreationData) (domain.MsgId, error) {
	// Validate text
	err := b.validator.Text(creationData.Text)
	if err != nil {
		return 0, err
	}

	// Validate pending files if present
	if err := b.validator.PendingFiles(creationData.PendingFiles); err != nil {
		return 0, err
	}

	// Always create message metadata first (without files)
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
		// Save file to storage
		filePath, err := b.mediaStorage.SaveFile(
			pendingFile.Data,
			string(board),
			fmt.Sprintf("%d", threadID),
			pendingFile.OriginalFilename,
		)
		if err != nil {
			// Cleanup: delete already saved files
			for _, savedPath := range savedFiles {
				b.mediaStorage.DeleteFile(savedPath)
			}
			return fmt.Errorf("failed to save file: %w", err)
		}
		savedFiles = append(savedFiles, filePath)

		// Create file metadata
		fileData := &domain.File{
			FilePath:         filePath,
			OriginalFilename: pendingFile.OriginalFilename,
			FileSizeBytes:    pendingFile.Size,
			MimeType:         pendingFile.MimeType,
			ImageWidth:       pendingFile.ImageWidth,
			ImageHeight:      pendingFile.ImageHeight,
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
			// Best effort: log errors but don't fail the operation
			if err := b.mediaStorage.DeleteFile(attachment.File.FilePath); err != nil {
				// In production, you might want to log this error
				// For now, we continue to delete other files
			}
		}
	}

	return nil
}
