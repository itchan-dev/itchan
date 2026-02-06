package domain

import (
	"time"
)

// to iterate thru layers: handler -> service -> storage
type BoardCreationData struct {
	Name          BoardName      `json:"name" validate:"required"`
	ShortName     BoardShortName `json:"short_name" validate:"required"`
	AllowedEmails *Emails        `json:"allowed_emails,omitempty"`
}

type BoardMetadata struct {
	Name                BoardName
	ShortName           BoardShortName
	CreatedAt           time.Time
	LastActivityAt      time.Time
	AllowedEmailDomains []string // nil means public board, non-empty means corporate board
}

type Board struct {
	BoardMetadata
	Threads []*Thread
}
