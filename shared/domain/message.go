package domain

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
)

type Attachments = pq.StringArray // to save into postgres. strings of pathes

type Message struct {
	Id          int64
	Author      User
	Text        string
	CreatedAt   time.Time
	ModifiedAt  time.Time
	Attachments *Attachments
	ThreadId    sql.NullInt64
}
