package domain

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

func (u *User) Domain() (string, error) {
	emailParts := strings.Split(u.Email, "@")
	if len(emailParts) != 2 {
		log.Printf("Cant split user email: %s", u.Email)
		return "", errors.New("Cant get user domain")
	}
	return emailParts[1], nil
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

func (t *Thread) String() string {
	s := fmt.Sprintf("[title:%s, board:%s, reply_count:%d, last_bumped:%v, messages:[", t.Title, t.Board, t.NumReplies, t.LastBumped)
	for i, msg := range t.Messages {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%v", msg)
	}
	return s + "]]\n"
}
