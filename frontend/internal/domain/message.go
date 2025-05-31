package frontend_domain

import (
	"fmt"
	"html/template"

	"github.com/itchan-dev/itchan/shared/domain"
)

type Reply struct {
	Board  domain.BoardShortName
	Thread domain.ThreadId
	From   domain.MsgId
	To     domain.MsgId
}

type Replies []Reply

type Message struct {
	domain.Message
	Text    template.HTML // overwrite domain.Message.Text
	Replies Replies
}

func (r *Reply) LinkTo() string {
	return fmt.Sprintf(`<a href="/%[1]s/%[2]d#p%[3]d" class="message-link" data-message-id="%[3]d" data-thread-id="%[2]d">>>%[2]d/%[3]d</a>`,
		r.Board, r.Thread, r.To)
}

func (r *Reply) LinkFrom() string {
	return fmt.Sprintf(`<a href="/%[1]s/%[2]d#p%[3]d" class="message-link" data-message-id="%[3]d" data-thread-id="%[2]d">>>%[2]d/%[3]d</a>`,
		r.Board, r.Thread, r.From)
}
func (r *Reply) LinkFromHTML() template.HTML {
	return template.HTML(r.LinkFrom())
}
