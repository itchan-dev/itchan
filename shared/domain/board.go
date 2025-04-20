package domain

import (
	"time"

	"github.com/lib/pq"
)

type Emails = pq.StringArray

type Board struct {
	Name          string
	ShortName     string
	Threads       []*Thread
	AllowedEmails *Emails
	CreatedAt     time.Time
	LastActivity  time.Time
}
