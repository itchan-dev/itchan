package handler

import (
	"github.com/itchan-dev/itchan/shared/logger"
	"html/template"

	"net/http"
)

// BlacklistUserHandler handles blacklist requests from the UI
func (h *Handler) BlacklistUserHandler(w http.ResponseWriter, r *http.Request) {
	// Parse form to get userId and reason
	if err := r.ParseForm(); err != nil {
		logger.Log.Error("parsing form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	userID := r.FormValue("userId")
	reason := r.FormValue("reason")
	referer := r.FormValue("referer")

	if userID == "" {
		http.Error(w, "Missing userId", http.StatusBadRequest)
		return
	}

	// Default redirect target
	targetURL := "/"
	if referer != "" {
		targetURL = referer
	}

	// Call API client
	err := h.APIClient.BlacklistUser(r, userID, reason)
	if err != nil {
		logger.Log.Error("blacklisting user via API", "error", err)
		h.redirectWithFlash(w, r, targetURL, flashCookieError, template.HTMLEscapeString(err.Error()))
		return
	}

	// Success - redirect with success message
	h.redirectWithFlash(w, r, targetURL, flashCookieSuccess, "User blacklisted successfully")
}
