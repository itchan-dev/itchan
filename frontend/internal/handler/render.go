package handler

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/itchan-dev/itchan/shared/logger"

	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/shared/domain"
)

// checkNotModified handles HTTP conditional GET requests using Last-Modified/If-Modified-Since.
// Returns true if a 304 Not Modified response was sent (caller should return early).
func checkNotModified(w http.ResponseWriter, r *http.Request, lastModified time.Time) bool {
	lastModified = lastModified.UTC().Truncate(time.Second)

	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Vary", "Cookie")
	w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))

	if ifModifiedSince := r.Header.Get("If-Modified-Since"); ifModifiedSince != "" {
		if t, err := http.ParseTime(ifModifiedSince); err == nil {
			if !lastModified.After(t.UTC().Truncate(time.Second)) {
				w.WriteHeader(http.StatusNotModified)
				return true
			}
		}
	}
	return false
}

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

// RenderMessage transforms a domain.Message into a frontend-specific view model.
func RenderMessage(message domain.Message) *frontend_domain.Message {
	renderedMessage := frontend_domain.Message{Message: message}
	renderedMessage.Text = template.HTML(message.Text)

	if renderedMessage.IsOp() {
		renderedMessage.Context.ExtraClasses = "op-post"
	} else {
		renderedMessage.Context.ExtraClasses = "reply-post"
	}

	return &renderedMessage
}

func RenderThread(thread domain.Thread) *frontend_domain.Thread {
	renderedThread := frontend_domain.Thread{Thread: thread, Messages: make([]*frontend_domain.Message, len(thread.Messages))}
	for i, msg := range thread.Messages {
		renderedThread.Messages[i] = RenderMessage(*msg)

		// Enrich OP messages (id=1) with thread-specific context
		if msg.IsOp() {
			renderedThread.Messages[i].Context.Subject = thread.Title
			renderedThread.Messages[i].Context.IsPinned = thread.IsPinned
		}
	}
	return &renderedThread
}

func RenderBoard(board domain.Board) *frontend_domain.Board {
	renderedBoard := frontend_domain.Board{Board: board, Threads: make([]*frontend_domain.Thread, len(board.Threads))}
	for i, thread := range board.Threads {
		renderedBoard.Threads[i] = RenderThread(*thread)
	}
	return &renderedBoard
}
