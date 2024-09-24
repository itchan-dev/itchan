package message

import (
	"github.com/itchan-dev/itchan/internal/domain"
)

type Message struct {
	storage   Storage
	validator Validator
}

type Storage interface {
	CreateMessage(author domain.User, text string, attachments []domain.Attachment) (int64, error)
	GetMessage(id int64) (*domain.Message, error)
	DeleteMessage(id int64) error
}

type Validator interface {
	Text(text string) error
}

func New(storage Storage, validator Validator) *Message {
	return &Message{storage, validator}
}

func (b *Message) Create(author domain.User, text string, attachments []domain.Attachment) (int64, error) {
	err := b.validator.Text(text)
	if err != nil {
		return 0, err
	}

	id, err := b.storage.CreateMessage(author, text, attachments)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (b *Message) Get(id int64) (*domain.Message, error) {
	message, err := b.storage.GetMessage(id)
	if err != nil {
		return nil, err
	}
	return message, nil
}

func (b *Message) Delete(id int64) error {
	return b.storage.DeleteMessage(id)
}
