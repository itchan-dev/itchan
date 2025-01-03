package domain

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type Attachments = pq.StringArray // to save into postgres. strings of pathes

type Message struct {
	Id          int64
	Author      User
	Text        string
	CreatedAt   time.Time
	Attachments *Attachments
	ThreadId    sql.NullInt64
}

// for debug
func (m *Message) String() string {
	s := fmt.Sprintf("[id:%d, author:%v, text:%s, created:%s, thread_id:%v, attachments:[", m.Id, m.Author, m.Text, m.CreatedAt.Format(time.StampMilli), m.ThreadId)
	if m.Attachments != nil {
		for i, atch := range *m.Attachments {
			if i > 0 {
				s += ", "
			}
			s += fmt.Sprintf("%s", atch)
		}
	}
	return s + "]]"
}
