package domain

import "github.com/lib/pq"

type (
	Email    = string
	Password = string
	UserId   = int64

	Emails         = pq.StringArray
	BoardName      = string
	BoardShortName = string

	ThreadTitle = string
	ThreadId    = int64

	MsgText      = string
	MsgId        = int64
	Replies      = []*Reply
	FileId       = int64
	AttachmentId = int64
)
