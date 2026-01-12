package domain

import "time"

type Credentials struct {
	Email    Email
	Password Password
}

type User struct {
	Id        UserId
	Email     Email
	PassHash  Password
	Admin     bool
	CreatedAt time.Time
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

// InviteCode represents an invite code in the system
type InviteCode struct {
	CodeHash   string
	CreatedBy  UserId
	CreatedAt  time.Time
	ExpiresAt  time.Time
	UsedBy     *UserId    // nil if unused
	UsedAt     *time.Time // nil if unused
}

// InviteCodeWithPlaintext is returned when generating a new invite
// It contains the plain-text code that should only be shown once to the creator
type InviteCodeWithPlaintext struct {
	PlainCode string
	InviteCode
}
