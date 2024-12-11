package utils

import (
	"unicode"
	"unicode/utf8"

	"github.com/itchan-dev/itchan/backend/internal/errors"
)

func IsLetter(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

type BoardNameValidator struct{}

func (e *BoardNameValidator) Name(name string) error {
	if utf8.RuneCountInString(name) > 10 {
		return &errors.ValidationError{Message: "name is too long"}
	}
	if !IsLetter(name) {
		return &errors.ValidationError{Message: "name should contain only letters"}
	}
	return nil
}

func (e *BoardNameValidator) ShortName(name string) error {
	if utf8.RuneCountInString(name) > 2 {
		return &errors.ValidationError{Message: "name is too long"}
	}
	if !IsLetter(name) {
		return &errors.ValidationError{Message: "name should contain only letters"}
	}
	return nil
}

func New() *BoardNameValidator {
	return &BoardNameValidator{}
}

type ThreadTitleValidator struct{}

func (e *ThreadTitleValidator) Title(name string) error {
	if utf8.RuneCountInString(name) > 50 {
		return &errors.ValidationError{Message: "name is too long"}
	}
	return nil
}

type MessageValidator struct{}

func (e *MessageValidator) Text(name string) error {
	if utf8.RuneCountInString(name) > 10_000 {
		return &errors.ValidationError{Message: "text is too long"}
	}
	return nil
}
