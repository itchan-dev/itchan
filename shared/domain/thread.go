package domain

import (
	"fmt"
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

func (t *Thread) String() string {
	s := fmt.Sprintf("[title:%s, board:%s, reply_count:%d, last_bumped:%v, messages:[", t.Title, t.Board, t.NumReplies, t.LastBumped)
	for i, msg := range t.Messages {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%v", msg)
	}
	return s + "]]"
}
