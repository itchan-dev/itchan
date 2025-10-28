package service

import (
	"github.com/itchan-dev/itchan/shared/domain"
)

type MessageService interface {
	Create(creationData domain.MessageCreationData) (domain.MsgId, error)
	Get(board domain.BoardShortName, id domain.MsgId) (domain.Message, error)
	Delete(board domain.BoardShortName, id domain.MsgId) error
}

type Message struct {
	storage   MessageStorage
	validator MessageValidator
}

type MessageStorage interface {
	CreateMessage(creationData domain.MessageCreationData) (domain.MsgId, error)
	GetMessage(board domain.BoardShortName, id domain.MsgId) (domain.Message, error)
	DeleteMessage(board domain.BoardShortName, id domain.MsgId) error
}

type MessageValidator interface {
	Text(text domain.MsgText) error
}

func NewMessage(storage MessageStorage, validator MessageValidator) MessageService {
	return &Message{storage, validator}
}

func (b *Message) Create(creationData domain.MessageCreationData) (domain.MsgId, error) {
	err := b.validator.Text(creationData.Text)
	if err != nil {
		return 0, err
	}

	id, err := b.storage.CreateMessage(creationData)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (b *Message) Get(board domain.BoardShortName, id domain.MsgId) (domain.Message, error) {
	message, err := b.storage.GetMessage(board, id)
	if err != nil {
		return domain.Message{}, err
	}
	return message, nil
}

func (b *Message) Delete(board domain.BoardShortName, id domain.MsgId) error {
	return b.storage.DeleteMessage(board, id)
}
