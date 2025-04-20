package service

// TODO: что-то сделать с domain.Message в Create

import (
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
)

type ThreadService interface {
	Create(title string, board string, msg *domain.Message) (int64, error)
	Get(id int64) (*domain.Thread, error)
	Delete(board string, id int64) error
}

type Thread struct {
	storage   ThreadStorage
	validator ThreadValidator
	config    config.Public
}

type ThreadStorage interface {
	CreateThread(title, board string, msg *domain.Message) (int64, error)
	GetThread(id int64) (*domain.Thread, error)
	DeleteThread(board string, id int64) error
	ThreadCount(board string) (int, error)
	LastThreadId(board string) (int64, error)
}

type ThreadValidator interface {
	Title(title string) error
}

func NewThread(storage ThreadStorage, validator ThreadValidator, config config.Public) ThreadService {
	return &Thread{storage, validator, config}
}

func (b *Thread) Create(title string, board string, msg *domain.Message) (int64, error) {
	err := b.validator.Title(title)
	if err != nil {
		return -1, err
	}
	threadId, err := b.storage.CreateThread(title, board, msg)
	if err != nil {
		return -1, err
	}
	if b.config.MaxThreadCount != nil {
		threadCount, err := b.storage.ThreadCount(board)
		if err != nil {
			return threadId, err
		}
		if threadCount > *b.config.MaxThreadCount {
			lastThreadId, err := b.storage.LastThreadId(board)
			if err != nil {
				return threadId, err
			}
			if err := b.Delete(board, lastThreadId); err != nil {
				return threadId, err
			}
		}
	}
	return threadId, nil
}

func (b *Thread) Get(id int64) (*domain.Thread, error) {
	thread, err := b.storage.GetThread(id)
	if err != nil {
		return nil, err
	}
	return thread, nil
}

func (b *Thread) Delete(board string, id int64) error {
	return b.storage.DeleteThread(board, id)
}
