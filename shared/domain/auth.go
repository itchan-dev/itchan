package domain

import "time"

type Credentials struct {
	Email    Email
	Password Password
}

type User struct {
	Id        UserId
	// Stored fields - encrypted and hashed email data
	EmailEncrypted []byte
	EmailDomain    string
	EmailHash      []byte
	PassHash       Password
	Admin          bool
	CreatedAt      time.Time
}

// SaveUserData contains the data needed to create a new user
type SaveUserData struct {
	Email    Email
	PassHash Password
	Admin    bool
}

type ConfirmationData struct {
	EmailHash            []byte // Email hash for lookup
	PasswordHash         Password
	ConfirmationCodeHash string
	Expires              time.Time
}

type BlacklistEntry struct {
	UserId        UserId
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
