package domain

import (
	"time"
)

// to iterate thru layers: handler -> service -> storage
type ThreadCreationData struct {
	Title     ThreadTitle
	Board     BoardShortName
	IsPinned  bool
	OpMessage MessageCreationData
}

type ThreadMetadata struct {
	Id           ThreadId
	Title        ThreadTitle
	Board        BoardShortName
	MessageCount int
	LastBumped   time.Time
	IsPinned     bool
}

type Thread struct {
	ThreadMetadata
	Messages []*Message // Change from []Message to []*Message
}
