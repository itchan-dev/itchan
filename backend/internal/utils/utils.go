package utils

import (
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/errors"
)

func IsLetter(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

type BoardNameValidator struct{ Сfg *config.Public }

func (e *BoardNameValidator) Name(name string) error {
	if utf8.RuneCountInString(name) > e.Сfg.BoardNameMaxLen {
		return &errors.ErrorWithStatusCode{Message: "Name is too long", StatusCode: 400}
	}
	if !IsLetter(name) {
		return &errors.ErrorWithStatusCode{Message: "Name should contain only letters", StatusCode: 400}
	}
	return nil
}

func (e *BoardNameValidator) ShortName(name string) error {
	if utf8.RuneCountInString(name) > e.Сfg.BoardShortNameMaxLen {
		return &errors.ErrorWithStatusCode{Message: "Name is too long", StatusCode: 400}
	}
	if !IsLetter(name) {
		return &errors.ErrorWithStatusCode{Message: "Name should contain only letters", StatusCode: 400}
	}
	return nil
}

func New(cfg *config.Public) *BoardNameValidator {
	return &BoardNameValidator{Сfg: cfg}
}

type ThreadTitleValidator struct{ Сfg *config.Public }

func (e *ThreadTitleValidator) Title(name string) error {
	if utf8.RuneCountInString(name) > e.Сfg.ThreadTitleMaxLen {
		return &errors.ErrorWithStatusCode{Message: "Name is too long", StatusCode: 400}
	}
	return nil
}

type MessageValidator struct{ Сfg *config.Public }

func (e *MessageValidator) Text(name string) error {
	if utf8.RuneCountInString(name) > e.Сfg.MessageTextMaxLen {
		return &errors.ErrorWithStatusCode{Message: "Text is too long", StatusCode: 400}
	}
	if len(name) <= e.Сfg.MessageTextMinLen {
		return &errors.ErrorWithStatusCode{Message: "Text is too short", StatusCode: 400}
	}
	return nil
}

func GenerateConfirmationCode(len int) string {
	code := uuid.NewString()
	return code[:len]
}
