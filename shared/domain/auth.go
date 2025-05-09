package domain

import "time"

type Credentials struct {
	Email    Email
	Password Password
}

type User struct {
	Id       UserId
	Email    Email
	PassHash Password
	Admin    bool
}

type ConfirmationData struct {
	Email                Email
	NewPassHash          Password
	ConfirmationCodeHash string
	Expires              time.Time
}
