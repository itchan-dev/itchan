package service

import (
	"github.com/itchan-dev/itchan/shared/domain"
)

// to mock service in tests
type BoardService interface {
	Create(name, shortName string) error
	Get(shortName string, page int) (*domain.Board, error)
	Delete(shortName string) error
}

type Board struct {
	storage       BoardStorage
	nameValidator BoardValidator
}

type BoardStorage interface {
	CreateBoard(name, shortName string) error
	GetBoard(shortName string, page int) (*domain.Board, error)
	DeleteBoard(shortName string) error
}

type BoardValidator interface {
	Name(name string) error
	ShortName(name string) error
}

func NewBoard(storage BoardStorage, validator BoardValidator) BoardService {
	return &Board{storage, validator}
}

func (b *Board) Create(name, shortName string) error {
	if err := b.nameValidator.Name(name); err != nil {
		return err
	}
	if err := b.nameValidator.ShortName(shortName); err != nil {
		return err
	}
	if err := b.storage.CreateBoard(name, shortName); err != nil {
		return err
	}

	return nil
}

func (b *Board) Get(shortName string, page int) (*domain.Board, error) {
	page = max(1, page)

	if err := b.nameValidator.ShortName(shortName); err != nil {
		return nil, err
	}

	board, err := b.storage.GetBoard(shortName, page)
	if err != nil {
		return nil, err
	}
	return board, nil
}

func (b *Board) Delete(shortName string) error {
	if err := b.nameValidator.ShortName(shortName); err != nil {
		return err
	}

	return b.storage.DeleteBoard(shortName)
}
