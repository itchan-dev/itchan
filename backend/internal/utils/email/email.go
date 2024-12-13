package email

import (
	"net/mail"

	"github.com/itchan-dev/itchan/backend/internal/errors"
)

type Email struct{}

func (e *Email) Confirm(email string) error { // to do
	return nil
}

func (e *Email) IsCorrect(email string) error {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return &errors.ErrorWithStatusCode{Message: err.Error(), StatusCode: 400}
	}
	return nil
}

func New() *Email {
	return &Email{}
}
