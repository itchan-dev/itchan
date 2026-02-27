package handler

import (
	"fmt"
	"net/http"

	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
	"github.com/itchan-dev/itchan/shared/utils"
)

// AdminGetHandler displays the admin panel with blacklisted users and referral stats.
func (h *Handler) AdminGetHandler(w http.ResponseWriter, r *http.Request) {
	page := utils.GetPage(r)

	blacklist, err := h.APIClient.GetBlacklistedUsers(r, page)
	var errMsg string
	if err != nil {
		logger.Log.Error("failed to get blacklisted users from API", "error", err)
		errMsg = fmt.Sprintf("Failed to load blacklisted users: %v", err)
		blacklist = api.BlacklistResponse{Users: []domain.BlacklistEntry{}, Page: page}
	}

	stats, err := h.APIClient.GetReferralStats(r)
	if err != nil {
		logger.Log.Error("failed to get referral stats from API", "error", err)
	}

	data := frontend_domain.AdminPageData{
		Blacklisted: frontend_domain.BlacklistedUsers{Users: blacklist.Users, Page: blacklist.Page},
		RefStats:    stats.Stats,
	}

	h.renderTemplateWithError(w, r, "admin.html", data, errMsg)
}

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
