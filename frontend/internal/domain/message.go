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

// MessageView contains rendering context that isn't in the Message struct.
// Templates can access Message fields directly and build URLs using printf.
type MessageView struct {
	*Message // Embed the base message data (includes Board, ThreadId, Id, Text, Attachments, Replies, etc.)

	// Rendering context (not in Message)
	ExtraClasses     string // CSS classes like "op-post", "reply-post", "message-preview"
	ShowDeleteButton bool   // Whether to show delete button (depends on user permissions)
	ShowReplyButton  bool   // Whether to show reply button (depends on page context)
	Subject          string // Optional subject line (usually only for OP, from thread title)
}
