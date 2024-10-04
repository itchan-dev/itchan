package scripts

import (
	"errors"
	"unicode"
	"unicode/utf8"
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
		return errors.New("name is too long")
	}
	if !IsLetter(name) {
		return errors.New("name should contain only letters")
	}
	return nil
}

func (e *BoardNameValidator) ShortName(name string) error {
	if utf8.RuneCountInString(name) > 2 {
		return errors.New("name is too long")
	}
	if !IsLetter(name) {
		return errors.New("name should contain only letters")
	}
	return nil
}

func New() *BoardNameValidator {
	return &BoardNameValidator{}
}
