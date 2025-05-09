package domain

import (
	"time"
)

// to iterate thru layers: handler -> service -> storage
type MessageCreationData struct {
	Board       BoardShortName
	ThreadId    MsgId
	Author      User
	CreatedAt   *time.Time
	Text        MsgText
	Attachments *Attachments
}

type MessageMetadata struct {
	Board      BoardShortName
	ThreadId   ThreadId
	Id         MsgId
	Author     User
	CreatedAt  time.Time
	Ordinal    int
	ModifiedAt time.Time
	Op         bool
}

type Message struct {
	MessageMetadata
	Text        MsgText
	Attachments *Attachments
}
