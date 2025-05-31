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

	// Attachment  = string
	Attachments = pq.StringArray // to save into postgres REDO TO STRING (attachment)
	MsgText     = string
	MsgId       = int64
	Replies     = []Reply
)
