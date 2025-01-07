package domain

import "time"

type User struct {
	Id       int64
	Email    string
	PassHash string
	Admin    bool
}

type ConfirmationData struct {
	Email                string
	NewPassHash          string
	ConfirmationCodeHash string
	Expires              time.Time
}
