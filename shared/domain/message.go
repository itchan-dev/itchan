package domain

import (
	"time"
)

// to iterate thru layers: handler -> service -> storage
type MessageCreationData struct {
	Board        BoardShortName
	ThreadId     MsgId
	Author       User
	Text         MsgText
	CreatedAt    *time.Time
	PendingFiles []*PendingFile // Files to be saved after message creation
	ReplyTo      *Replies
}

type MessageMetadata struct {
	Board      BoardShortName
	ThreadId   ThreadId
	Id         MsgId
	Author     User
	Op         bool
	Ordinal    int
	Page       int // Page number where this message appears (calculated from Ordinal)
	Replies    Replies
	CreatedAt  time.Time
	ModifiedAt time.Time
}

type Message struct {
	MessageMetadata
	Text        string
	Attachments Attachments
}

type Reply struct {
	Board        BoardShortName
	FromThreadId ThreadId
	ToThreadId   ThreadId
	From         MsgId
	To           MsgId
	FromOrdinal  int // Ordinal of the sender message
	FromPage     int // Page where the sender message is located (calculated from FromOrdinal)
	CreatedAt    time.Time
}
