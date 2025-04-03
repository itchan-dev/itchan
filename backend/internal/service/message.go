package service

import (
	"github.com/itchan-dev/itchan/shared/domain"
)

type MessageService interface {
	Create(board string, author *domain.User, text string, attachments *domain.Attachments, threadId int64) (int64, error)
	Get(id int64) (*domain.Message, error)
	Delete(board string, id int64) error
}

type Message struct {
	storage   MessageStorage
	validator MessageValidator
}

type MessageStorage interface {
	CreateMessage(board string, author *domain.User, text string, attachments *domain.Attachments, threadId int64) (int64, error)
	GetMessage(id int64) (*domain.Message, error)
	DeleteMessage(board string, id int64) error
}

type MessageValidator interface {
	Text(text string) error
}

func NewMessage(storage MessageStorage, validator MessageValidator) MessageService {
	return &Message{storage, validator}
}

func (b *Message) Create(board string, author *domain.User, text string, attachments *domain.Attachments, threadId int64) (int64, error) {
	err := b.validator.Text(text)
	if err != nil {
		return 0, err
	}

	id, err := b.storage.CreateMessage(board, author, text, attachments, threadId)
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

func (b *Message) Delete(board string, id int64) error {
	return b.storage.DeleteMessage(board, id)
}
