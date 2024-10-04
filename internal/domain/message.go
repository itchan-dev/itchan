package domain

type Attachment string

type Message struct {
	Id          int64
	Author      User
	Text        string
	CreatedAt   int64
	Attachments []*Attachment
	ThreadId    int
}
