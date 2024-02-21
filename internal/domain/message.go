package domain

type Attachment struct {
}

type Message struct {
	author      User
	Text        string
	createdAt   timestamp
	Id          int64
	Attachments []Attachment
}
