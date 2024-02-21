package email

import "net/mail"

type Email struct{}

func (e *Email) Confirm(email string) error { // to do
	return nil
}

func (e *Email) IsCorrect(email string) error {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return err
	}
	return nil
}

func New() *Email {
	return &Email{}
}
