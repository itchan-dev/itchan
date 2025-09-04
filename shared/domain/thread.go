package domain

import (
	"time"
)

// to iterate thru layers: handler -> service -> storage
type ThreadCreationData struct {
	Title     ThreadTitle
	Board     BoardShortName
	IsSticky  bool
	OpMessage MessageCreationData
}

type ThreadMetadata struct {
	Id         ThreadId
	Title      ThreadTitle
	Board      BoardShortName
	NumReplies int
	LastBumped time.Time
	IsSticky   bool
}

type Thread struct {
	ThreadMetadata
	Messages []*Message  // Change from []Message to []*Message
}
