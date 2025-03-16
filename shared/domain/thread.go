package domain

import (
	"time"
)

type Thread struct {
	Title      string
	Messages   []*Message // all other metainfo = 1st message metainfo
	Board      string
	NumReplies uint
	LastBumped time.Time
}

func (t *Thread) Id() int64 {
	return t.Messages[0].Id
}
