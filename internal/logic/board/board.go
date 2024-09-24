package board

import (
	"github.com/itchan-dev/itchan/internal/domain"
)

type Board struct {
	storage   Storage
	validator Validator
}

type Storage interface {
	CreateBoard(name, shortName string) error
	GetBoard(shortName string) (*domain.Board, error)
	DeleteBoard(shortName string) error
}

type Validator interface {
	Name(name string) error
	ShortName(name string) error
}

func New(storage Storage, validator Validator) *Board {
	return &Board{storage, validator}
}

func (b *Board) Create(name, shortName string) error {
	err := b.validator.Name(name)
	if err != nil {
		return err
	}
	err = b.validator.ShortName(shortName)
	if err != nil {
		return err
	}

	err = b.storage.CreateBoard(name, shortName)
	if err != nil {
		return err
	}
	return nil
}

func (b *Board) Get(shortName string) (*domain.Board, error) {
	err := b.validator.ShortName(shortName)
	if err != nil {
		return nil, err
	}

	board, err := b.storage.GetBoard(shortName)
	if err != nil {
		return nil, err
	}
	return board, nil
}

func (b *Board) Delete(shortName string) error {
	err := b.validator.ShortName(shortName)
	if err != nil {
		return err
	}

	return b.storage.DeleteBoard(shortName)
}
