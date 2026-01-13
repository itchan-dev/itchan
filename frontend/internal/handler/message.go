package handler

import (
	"github.com/itchan-dev/itchan/shared/logger"
	"fmt"
	"html/template"
	"io"

	"net/http"

	"github.com/gorilla/mux"
	mw "github.com/itchan-dev/itchan/shared/middleware"
)

func (h *Handler) MessageDeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boardShortName := vars["board"]
	threadId := vars["thread"]
	messageId := vars["message"]
	targetURL := fmt.Sprintf("/%s/%s", boardShortName, threadId)

	err := h.APIClient.DeleteMessage(r, boardShortName, threadId, messageId)
	if err != nil {
		logger.Log.Error("deleting message via API", "error", err)
		h.redirectWithFlash(w, r, targetURL, flashCookieError, template.HTMLEscapeString(err.Error()))
		return
	}

	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}

// MessagePreviewHandler proxies message API requests for JavaScript previews.
func (h *Handler) MessagePreviewHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	board := vars["board"]
	threadId := vars["thread"]
	messageId := vars["message"]

	resp, err := h.APIClient.GetMessage(r, board, threadId, messageId)
	if err != nil {
		http.Error(w, "Internal error: backend unavailable", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Forward headers from the backend response to the client.
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)

	// Copy the response body directly to the client.
	if _, err := io.Copy(w, resp.Body); err != nil {
		logger.Log.Error("copying response body for message preview", "error", err)
	}
}

// MessagePreviewHTMLHandler returns rendered HTML for message previews.
func (h *Handler) MessagePreviewHTMLHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	board := vars["board"]
	threadId := vars["thread"]
	messageId := vars["message"]

	// Fetch message data from backend API
	messageData, err := h.APIClient.GetMessageParsed(r, board, threadId, messageId)
	if err != nil {
		logger.Log.Error("fetching message from API", "error", err)
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// Convert to frontend domain types (adds HTMLLinkFrom method to Replies)
	renderedMessage := RenderMessage(*messageData)

	// Add message-preview class
	renderedMessage.Context.ExtraClasses += " message-preview"

	// Create view data dict - consistent with template pattern
	viewData := map[string]any{
		"Message": renderedMessage,
		"User":    mw.GetUserFromContext(r),
	}

	// Render the post template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl := h.Templates["partials"]
	if tmpl == nil {
		logger.Log.Error(": partials template not found in templates map")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "post", viewData); err != nil {
		logger.Log.Error("rendering post template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
