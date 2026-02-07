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

	threadID, createdAt, err := b.storage.CreateThread(creationData)
	if err != nil {
		return -1, err
	}

	opMessageData := creationData.OpMessage
	opMessageData.Board = creationData.Board
	opMessageData.ThreadId = threadID
	opMessageData.CreatedAt = &createdAt

	_, err = b.messageService.Create(opMessageData)
	if err != nil {
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
	err := b.storage.DeleteThread(board, id)
	if err != nil {
		return err
	}

	// Best effort: log errors but don't fail the operation
	if err := b.mediaStorage.DeleteThread(string(board), fmt.Sprintf("%d", id)); err != nil {
	}

	return nil
}

func (b *Thread) TogglePinned(board domain.BoardShortName, id domain.ThreadId) (bool, error) {
	return b.storage.TogglePinnedStatus(board, id)
}
