package domain

import (
	"io"
	"time"
)

// PendingFile represents a file upload that hasn't been saved to storage yet
type PendingFile struct {
	Data             io.Reader
	OriginalFilename string
	Size             int64
	MimeType         string
	ImageWidth       *int
	ImageHeight      *int
}

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
	CreatedAt    time.Time
}
