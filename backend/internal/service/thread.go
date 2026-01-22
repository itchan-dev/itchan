package service

import (
	"fmt"
	"time"

	"github.com/itchan-dev/itchan/backend/internal/storage/fs"
	"github.com/itchan-dev/itchan/shared/domain"
)

type ThreadService interface {
	// Create returns only ThreadId - OP message always has Id=1
	Create(creationData domain.ThreadCreationData) (domain.ThreadId, error)
	Get(board domain.BoardShortName, id domain.ThreadId, page int) (domain.Thread, error)
	Delete(board domain.BoardShortName, id domain.ThreadId) error
	TogglePinned(board domain.BoardShortName, id domain.ThreadId) (bool, error)
}

type Thread struct {
	storage        ThreadStorage
	validator      ThreadValidator
	messageService MessageService
	mediaStorage   fs.MediaStorage
}

type ThreadStorage interface {
	CreateThread(creationData domain.ThreadCreationData) (domain.ThreadId, time.Time, error)
	GetThread(board domain.BoardShortName, id domain.ThreadId, page int) (domain.Thread, error)
	DeleteThread(board domain.BoardShortName, id domain.ThreadId) error
	ThreadCount(board domain.BoardShortName) (int, error)
	LastThreadId(board domain.BoardShortName) (domain.ThreadId, error)
	TogglePinnedStatus(board domain.BoardShortName, threadId domain.ThreadId) (bool, error)
}

type ThreadValidator interface {
	Title(title domain.ThreadTitle) error
}

func NewThread(storage ThreadStorage, validator ThreadValidator, messageService MessageService, mediaStorage fs.MediaStorage) ThreadService {
	return &Thread{
		storage:        storage,
		validator:      validator,
		messageService: messageService,
		mediaStorage:   mediaStorage,
	}
}

func (b *Thread) Create(creationData domain.ThreadCreationData) (domain.ThreadId, error) {
	// Validate title
	err := b.validator.Title(creationData.Title)
	if err != nil {
		return -1, err
	}

	// Step 1: Create the thread record (just metadata)
	threadID, createdAt, err := b.storage.CreateThread(creationData)
	if err != nil {
		return -1, err
	}

	// Step 2: Prepare OP message creation data
	opMessageData := creationData.OpMessage
	opMessageData.Board = creationData.Board
	opMessageData.ThreadId = threadID
	opMessageData.CreatedAt = &createdAt // Same timestamp as thread

	// Step 3: Create OP message using Message service (handles files automatically)
	// OP message always gets Id=1 (first message in thread)
	_, err = b.messageService.Create(opMessageData)
	if err != nil {
		// Cleanup: delete the thread since OP message creation failed
		b.storage.DeleteThread(creationData.Board, threadID)
		return -1, fmt.Errorf("failed to create OP message: %w", err)
	}

	// Thread cleanup is now handled by ThreadGarbageCollector in the background
	return threadID, nil
}

func (b *Thread) Get(board domain.BoardShortName, id domain.ThreadId, page int) (domain.Thread, error) {
	thread, err := b.storage.GetThread(board, id, page)
	if err != nil {
		return domain.Thread{}, err
	}
	return thread, nil
}

func (b *Thread) Delete(board domain.BoardShortName, id domain.ThreadId) error {
	// Delete the thread from storage (DB will cascade delete all messages and attachments)
	err := b.storage.DeleteThread(board, id)
	if err != nil {
		return err
	}

	// Delete all files for this thread from filesystem
	// Best effort: log errors but don't fail the operation
	if err := b.mediaStorage.DeleteThread(string(board), fmt.Sprintf("%d", id)); err != nil {
		// In production, you might want to log this error
		// For now, we continue as the DB records are already deleted
	}

	return nil
}

func (b *Thread) TogglePinned(board domain.BoardShortName, id domain.ThreadId) (bool, error) {
	return b.storage.TogglePinnedStatus(board, id)
}
