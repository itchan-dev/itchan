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
	PasswordHash         Password
	ConfirmationCodeHash string
	Expires              time.Time
}

type BlacklistEntry struct {
	UserId        UserId
	Email         Email
	BlacklistedAt time.Time
	Reason        string
	BlacklistedBy UserId
}
