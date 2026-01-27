package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/itchan-dev/itchan/shared/logger"
)

func (u *User) GetEmailDomain() (string, error) {
	// Return the stored domain (populated from encrypted email storage)
	if u.EmailDomain == "" {
		logger.Log.Error("user email domain is empty", "userId", u.Id)
		return "", errors.New("email domain not available")
	}
	return u.EmailDomain, nil
}

// for debug
func (m *Message) String() string {
	s := fmt.Sprintf("[id:%d, author:%v, text:%s, created:%s, thread_id:%v, attachments:[", m.Id, m.Author, m.Text, m.CreatedAt.Format(time.StampMilli), m.ThreadId)
	if m.Attachments != nil {
		for i, atch := range m.Attachments {
			if i > 0 {
				s += ", "
			}
			s += fmt.Sprintf("%+v", atch)
		}
	}
	return s + "]]"
}

func (t *Thread) String() string {
	s := fmt.Sprintf("[title:%s, board:%s, message_count:%d, last_bumped:%v, messages:[", t.Title, t.Board, t.MessageCount, t.LastBumped)
	for i, msg := range t.Messages {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%v", msg)
	}
	return s + "]]\n"
}
