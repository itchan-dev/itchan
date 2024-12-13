package service

// TODO: что-то сделать с domain.Message в Create

import (
	"github.com/itchan-dev/itchan/shared/domain"
)

type ThreadService interface {
	Create(title string, board string, msg *domain.Message) (*domain.Thread, error)
	Get(id int64) (*domain.Thread, error)
	Delete(board string, id int64) error
}

type Thread struct {
	storage   ThreadStorage
	validator ThreadValidator
}

type ThreadStorage interface {
	CreateThread(title, board string, msg *domain.Message) (*domain.Thread, error)
	GetThread(id int64) (*domain.Thread, error)
	DeleteThread(board string, id int64) error
}

type ThreadValidator interface {
	Title(title string) error
}

func NewThread(storage ThreadStorage, validator ThreadValidator) ThreadService {
	return &Thread{storage, validator}
}

func (b *Thread) Create(title string, board string, msg *domain.Message) (*domain.Thread, error) {
	err := b.validator.Title(title)
	if err != nil {
		return nil, err
	}

	thread, err := b.storage.CreateThread(title, board, msg)
	if err != nil {
		return nil, err
	}
	return thread, nil
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
