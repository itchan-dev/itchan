package domain

import "time"

type User struct {
	Id                   int64
	Email                string
	PassHash             string
	Admin                bool
	ConfirmationCodeHash string
	ConfirmationExpires  time.Time
	IsConfirmed          bool
}
