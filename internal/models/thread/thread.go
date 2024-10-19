package thread

// TODO: что-то сделать с domain.Message в Create

import (
	"github.com/itchan-dev/itchan/internal/domain"
)

type Thread struct {
	storage   Storage
	validator Validator
}

type Storage interface {
	CreateThread(title, board string, msg *domain.Message) (*domain.Thread, error)
	GetThread(id int64) (*domain.Thread, error)
	DeleteThread(board string, id int64) error
}

type Validator interface {
	Title(title string) error
}

func New(storage Storage, validator Validator) *Thread {
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
