package frontend_domain

import (
	"html/template"

	"github.com/itchan-dev/itchan/shared/domain"
)

// RenderContext contains presentation-specific fields for rendering messages.
type RenderContext struct {
	ExtraClasses string // CSS classes: "op-post", "reply-post", "message-preview"
	Subject      string // Subject line (thread title for OP messages)
	IsPinned     bool   // Whether the parent thread is pinned (only relevant for OP messages)
}

// Message wraps domain.Message with frontend-specific fields.
// Text is overwritten for HTML safety. Replies and Page come from embedded struct.
type Message struct {
	domain.Message
	Text    template.HTML
	Context RenderContext
}
