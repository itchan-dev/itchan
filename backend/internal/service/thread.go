package service

// TODO: что-то сделать с domain.Message в Create

import (
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
)

type ThreadService interface {
	Create(creationData domain.ThreadCreationData) (domain.MsgId, error)
	Get(board domain.BoardShortName, id domain.MsgId) (domain.Thread, error)
	Delete(board domain.BoardShortName, id domain.MsgId) error
}

type Thread struct {
	storage   ThreadStorage
	validator ThreadValidator
	config    config.Public
}

type ThreadStorage interface {
	CreateThread(creationData domain.ThreadCreationData) (domain.MsgId, error)
	GetThread(board domain.BoardShortName, id domain.ThreadId) (domain.Thread, error)
	DeleteThread(board domain.BoardShortName, id domain.MsgId) error
	ThreadCount(board domain.BoardShortName) (int, error)
	LastThreadId(board domain.BoardShortName) (domain.MsgId, error)
}

type ThreadValidator interface {
	Title(title domain.ThreadTitle) error
}

func NewThread(storage ThreadStorage, validator ThreadValidator, config config.Public) ThreadService {
	return &Thread{storage, validator, config}
}

func (b *Thread) Create(creationData domain.ThreadCreationData) (domain.MsgId, error) {
	err := b.validator.Title(creationData.Title)
	if err != nil {
		return -1, err
	}
	threadId, err := b.storage.CreateThread(creationData)
	if err != nil {
		return -1, err
	}
	if b.config.MaxThreadCount != nil {
		threadCount, err := b.storage.ThreadCount(creationData.Board)
		if err != nil {
			return threadId, err
		}
		if threadCount > *b.config.MaxThreadCount {
			lastThreadId, err := b.storage.LastThreadId(creationData.Board)
			if err != nil {
				return threadId, err
			}
			if err := b.Delete(creationData.Board, lastThreadId); err != nil {
				return threadId, err
			}
		}
	}
	return threadId, nil
}

func (b *Thread) Get(board domain.BoardShortName, id domain.ThreadId) (domain.Thread, error) {
	thread, err := b.storage.GetThread(board, id)
	if err != nil {
		return domain.Thread{}, err
	}
	return thread, nil
}

func (b *Thread) Delete(board domain.BoardShortName, id domain.MsgId) error {
	return b.storage.DeleteThread(board, id)
}
