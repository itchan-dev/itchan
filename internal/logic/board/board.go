package board

import (
	"github.com/itchan-dev/itchan/internal/domain"
)

type Board struct {
	storage     Storage
	nameChecker NameChecker
}

type Storage interface {
	CreateBoard(name, shortName string) (*domain.Board, error)
	GetBoard(shortName string) (*domain.Board, error)
	DeleteBoard(shortName string) error
}

type NameChecker interface {
	Check(name string) error
	CheckShort(name string) error
}

func New(storage Storage, nameChecker NameChecker) *Board {
	return &Board{storage, nameChecker}
}

func (b *Board) Create(name, shortName string) (*domain.Board, error) {
	err := b.nameChecker.Check(name)
	if err != nil {
		return nil, err
	}
	err = b.nameChecker.CheckShort(shortName)
	if err != nil {
		return nil, err
	}

	board, err := b.storage.CreateBoard(name, shortName)
	if err != nil {
		return nil, err
	}
	return board, nil
}

func (b *Board) Get(shortName string) (*domain.Board, error) {
	err := b.nameChecker.CheckShort(shortName)
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
	err := b.nameChecker.CheckShort(shortName)
	if err != nil {
		return err
	}

	return b.storage.DeleteBoard(shortName)
}
