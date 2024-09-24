package domain

type Attachment struct {
}

type Message struct {
	Id          int64
	Author      User
	Text        string
	CreatedAt   timestamp
	Attachments []Attachment
	Thread      *Thread
}
