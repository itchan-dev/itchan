package domain

import (
	"time"
)

// to iterate thru layers: handler -> service -> storage
type BoardCreationData struct {
	Name          BoardName
	ShortName     BoardShortName
	AllowedEmails *Emails
}

type BoardMetadata struct {
	Name          BoardName
	ShortName     BoardShortName
	AllowedEmails *Emails
	CreatedAt     time.Time
	LastActivity  time.Time
}

type Board struct {
	BoardMetadata
	Threads []Thread
}
