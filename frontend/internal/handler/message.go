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
