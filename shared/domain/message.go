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
	Id         MsgId // Per-thread sequential (1, 2, 3...) - id=1 is OP
	Author     User
	Page       int // Page number where this message appears (calculated from Id)
	Replies    Replies
	CreatedAt  time.Time
	ModifiedAt time.Time
}

// IsOp returns true if this message is the opening post (first message in thread)
func (m *MessageMetadata) IsOp() bool {
	return m.Id == 1
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
	From         MsgId // Per-thread sequential ID (also serves as ordinal)
	To           MsgId
	FromPage     int // Page where the sender message is located (calculated from From)
	CreatedAt    time.Time
}
