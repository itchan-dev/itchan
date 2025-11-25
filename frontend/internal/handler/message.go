package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func (h *Handler) MessageDeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boardShortName := vars["board"]
	threadId := vars["thread"]
	messageId := vars["message"]
	targetURL := fmt.Sprintf("/%s/%s", boardShortName, threadId)

	err := h.APIClient.DeleteMessage(r, boardShortName, threadId, messageId)
	if err != nil {
		log.Printf("Error deleting message via API: %v", err)
		redirectWithParams(w, r, targetURL, map[string]string{"error": err.Error()})
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
		log.Printf("Error copying response body for message preview: %v", err)
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
		log.Printf("Error fetching message from API: %v", err)
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// Convert to frontend domain types (adds HTMLLinkFrom method to Replies)
	renderedMessage := RenderMessage(*messageData)

	// Determine extra classes based on OP status
	extraClasses := "reply-post message-preview"
	if messageData.Op {
		extraClasses = "op-post message-preview"
	}

	// Prepare view model - no delete button or reply button for previews
	viewData := PrepareMessageView(renderedMessage, MessageViewContext{
		ShowDeleteButton: false,
		ShowReplyButton:  false,
		ExtraClasses:     extraClasses,
		Subject:          "", // Previews don't show subject
	})

	// Render the post template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl := h.Templates["partials"]
	if tmpl == nil {
		log.Printf("Error: partials template not found in templates map")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "post", viewData); err != nil {
		log.Printf("Error rendering post template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
