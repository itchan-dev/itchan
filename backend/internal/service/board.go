package service

import (
	"github.com/itchan-dev/itchan/shared/domain"
)

type BoardService interface {
	Create(creationData domain.BoardCreationData) error
	Get(shortName domain.BoardShortName, page int) (domain.Board, error)
	Delete(shortName domain.BoardShortName) error
	GetAll(user domain.User) ([]domain.Board, error)
}

type Board struct {
	storage       BoardStorage
	nameValidator BoardValidator
}

type BoardStorage interface {
	CreateBoard(creationData domain.BoardCreationData) error
	GetBoard(shortName domain.BoardShortName, page int) (domain.Board, error)
	DeleteBoard(shortName domain.BoardShortName) error
	GetBoardsByUser(user domain.User) ([]domain.Board, error)
}

type BoardValidator interface {
	Name(name domain.BoardName) error
	ShortName(name domain.BoardShortName) error
}

func NewBoard(storage BoardStorage, validator BoardValidator) BoardService {
	return &Board{storage, validator}
}

func (b *Board) Create(creationData domain.BoardCreationData) error {
	if err := b.nameValidator.Name(creationData.Name); err != nil {
		return err
	}
	if err := b.nameValidator.ShortName(creationData.ShortName); err != nil {
		return err
	}
	if err := b.storage.CreateBoard(creationData); err != nil {
		return err
	}

	return nil
}

func (b *Board) Get(shortName domain.BoardShortName, page int) (domain.Board, error) {
	page = max(1, page)

	if err := b.nameValidator.ShortName(shortName); err != nil {
		return domain.Board{}, err
	}

	board, err := b.storage.GetBoard(shortName, page)
	if err != nil {
		return domain.Board{}, err
	}
	return board, nil
}

func (b *Board) GetAll(user domain.User) ([]domain.Board, error) {
	return b.storage.GetBoardsByUser(user)
}

func (b *Board) Delete(shortName domain.BoardShortName) error {
	if err := b.nameValidator.ShortName(shortName); err != nil {
		return err
	}

	return b.storage.DeleteBoard(shortName)
}
