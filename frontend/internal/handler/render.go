package handler

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/itchan-dev/itchan/shared/logger"

	"net/http"

	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/shared/domain"
)

// renderTemplate executes the given template with the provided data.
func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl, ok := h.Templates[name]
	if !ok {
		http.Error(w, fmt.Sprintf("Template %s not found", name), http.StatusInternalServerError)
		return
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, data); err != nil {
		logger.Log.Error("error executing template", "template", name, "error", err)
		http.Error(w, "Internal Server Error rendering template", http.StatusInternalServerError)
		return
	}

	_, _ = buf.WriteTo(w)
}

// RenderReply transforms a domain.Reply into a frontend-specific view model.
func RenderReply(reply domain.Reply) *frontend_domain.Reply {
	return &frontend_domain.Reply{Reply: reply}
}

// RenderMessage transforms a domain.Message into a frontend-specific view model.
func RenderMessage(message domain.Message) *frontend_domain.Message {
	renderedMessage := frontend_domain.Message{Message: message, Replies: make(frontend_domain.Replies, len(message.Replies))}
	renderedMessage.Text = template.HTML(message.Text)

	if renderedMessage.Op {
		renderedMessage.Context.ExtraClasses = "op-post"
	} else {
		renderedMessage.Context.ExtraClasses = "reply-post"
	}

	for i, reply := range message.Replies {
		renderedMessage.Replies[i] = RenderReply(*reply)
	}
	return &renderedMessage
}

// RenderThread transforms a domain.Thread into a frontend-specific view model.
func RenderThread(thread domain.Thread) *frontend_domain.Thread {
	renderedThread := frontend_domain.Thread{Thread: thread, Messages: make([]*frontend_domain.Message, len(thread.Messages))}
	for i, msg := range thread.Messages {
		renderedThread.Messages[i] = RenderMessage(*msg)

		// Enrich OP messages with thread-specific context
		if msg.Op {
			renderedThread.Messages[i].Context.Subject = thread.Title
		}
	}
	return &renderedThread
}

// RenderBoard transforms a domain.Board into a frontend-specific view model.
func RenderBoard(board domain.Board) *frontend_domain.Board {
	renderedBoard := frontend_domain.Board{Board: board, Threads: make([]*frontend_domain.Thread, len(board.Threads))}
	for i, thread := range board.Threads {
		renderedBoard.Threads[i] = RenderThread(*thread)
	}
	return &renderedBoard
}
