package frontend_domain

import (
	"fmt"
	"html/template"

	"github.com/itchan-dev/itchan/shared/domain"
)

// To add methods that is not replated to backend
type Reply struct {
	domain.Reply
}

type Replies = []*Reply

func (r *Reply) LinkTo() string {
	return fmt.Sprintf(`<a href="/%[1]s/%[2]d#p%[3]d" class="message-link" data-board="%[1]s" data-message-id="%[3]d" data-thread-id="%[2]d">>>%[2]d/%[3]d</a>`,
		r.Board, r.ToThreadId, r.To)
}

func (r *Reply) LinkFrom() string {
	return fmt.Sprintf(`<a href="/%[1]s/%[2]d#p%[3]d" class="message-link" data-board="%[1]s" data-message-id="%[3]d" data-thread-id="%[2]d">>>%[2]d/%[3]d</a>`,
		r.Board, r.FromThreadId, r.From)
}

func (r *Reply) HTMLLinkFrom() template.HTML {
	return template.HTML(r.LinkFrom())
}

type Message struct {
	domain.Message
	Text    template.HTML // overwrite domain.Message.Text
	Replies Replies
}
