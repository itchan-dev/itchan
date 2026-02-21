package handler

import (
	"fmt"
	"net/http"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
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

	if userID == "" {
		http.Error(w, "Missing userId", http.StatusBadRequest)
		return
	}

	// Use HTTP Referer header for redirect, fallback to home
	targetURL := r.Header.Get("Referer")
	if targetURL == "" {
		targetURL = "/"
	}

	// Call API client
	err := h.APIClient.BlacklistUser(r, userID, reason)
	if err != nil {
		logger.Log.Error("blacklisting user via API", "error", err)
		h.redirectWithFlash(w, r, targetURL, flashCookieError, err.Error())
		return
	}

	h.redirectWithFlash(w, r, targetURL, flashCookieSuccess, "User blacklisted successfully")
}

// AdminGetHandler displays the admin panel with blacklisted users
func (h *Handler) AdminGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		BlacklistedUsers []domain.BlacklistEntry
	}

	users, err := h.APIClient.GetBlacklistedUsers(r)
	var errMsg string
	if err != nil {
		logger.Log.Error("failed to get blacklisted users from API", "error", err)
		errMsg = fmt.Sprintf("Failed to load blacklisted users: %v", err)
		users = []domain.BlacklistEntry{}
	}
	templateData.BlacklistedUsers = users

	h.renderTemplateWithError(w, r, "admin.html", templateData, errMsg)
}

// UnblacklistUserHandler removes a user from the blacklist
func (h *Handler) UnblacklistUserHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		logger.Log.Error("parsing form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	userID := r.FormValue("userId")
	if userID == "" {
		http.Error(w, "Missing userId", http.StatusBadRequest)
		return
	}

	err := h.APIClient.UnblacklistUser(r, userID)
	if err != nil {
		logger.Log.Error("unblacklisting user via API", "error", err)
		h.redirectWithFlash(w, r, "/admin", flashCookieError, err.Error())
		return
	}

	h.redirectWithFlash(w, r, "/admin", flashCookieSuccess, "User removed from blacklist")
}
