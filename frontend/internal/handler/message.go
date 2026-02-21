package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/shared/logger"
)

func (h *Handler) MessageDeleteHandler(w http.ResponseWriter, r *http.Request) {
	boardShortName := chi.URLParam(r, "board")
	threadId := chi.URLParam(r, "thread")
	messageId := chi.URLParam(r, "message")
	targetURL := fmt.Sprintf("/%s/%s", boardShortName, threadId)

	err := h.APIClient.DeleteMessage(r, boardShortName, threadId, messageId)
	if err != nil {
		logger.Log.Error("deleting message via API", "error", err)
		h.redirectWithFlash(w, r, targetURL, flashCookieError, err.Error())
		return
	}

	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}

// MessagePreviewHandler proxies message API requests for JavaScript previews.
func (h *Handler) MessagePreviewHandler(w http.ResponseWriter, r *http.Request) {
	board := chi.URLParam(r, "board")
	threadId := chi.URLParam(r, "thread")
	messageId := chi.URLParam(r, "message")

	resp, err := h.APIClient.GetMessage(r, board, threadId, messageId)
	if err != nil {
		http.Error(w, "Internal error: backend unavailable", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		logger.Log.Error("copying response body for message preview", "error", err)
	}
}

// MessagePreviewHTMLHandler returns rendered HTML for message previews.
func (h *Handler) MessagePreviewHTMLHandler(w http.ResponseWriter, r *http.Request) {
	board := chi.URLParam(r, "board")
	threadId := chi.URLParam(r, "thread")
	messageId := chi.URLParam(r, "message")

	// Fetch message data from backend API
	messageData, err := h.APIClient.GetMessageParsed(r, board, threadId, messageId)
	if err != nil {
		logger.Log.Error("fetching message from API", "error", err)
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// Convert to frontend domain types (page already calculated by backend)
	renderedMessage := renderMessage(*messageData)
	renderedMessage.Context.ExtraClasses += " message-preview"

	common := h.initCommonTemplateData(w, r)
	viewData := frontend_domain.PostData{
		Message: renderedMessage,
		Common:  &common,
	}

	tmpl, ok := h.getTemplate("partials")
	if !ok {
		logger.Log.Error("partials template not found in templates map")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	buf := new(bytes.Buffer)
	if err := tmpl.ExecuteTemplate(buf, "post", viewData); err != nil {
		logger.Log.Error("rendering post template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}
