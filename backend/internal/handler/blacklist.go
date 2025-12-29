package handler

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

// BlacklistUser handles POST /v1/admin/users/:userId/blacklist
func (h *Handler) BlacklistUser(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL parameter
	userIdStr := mux.Vars(r)["userId"]
	userId, err := strconv.ParseInt(userIdStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Get admin user from context (needed for blacklistedBy field)
	admin := mw.GetUserFromContext(r)

	// Parse request body
	var req api.BlacklistUserRequest
	if err := utils.DecodeValidate(r.Body, &req); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	// Blacklist the user (service handles cache update)
	if err := h.auth.BlacklistUser(userId, req.Reason, admin.Id); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("User blacklisted successfully"))
}

// UnblacklistUser handles DELETE /v1/admin/users/:userId/blacklist
func (h *Handler) UnblacklistUser(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from URL parameter
	userIdStr := mux.Vars(r)["userId"]
	userId, err := strconv.ParseInt(userIdStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Unblacklist the user (service handles cache update)
	if err := h.auth.UnblacklistUser(userId); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("User unblacklisted successfully"))
}

// RefreshBlacklistCache handles POST /v1/admin/blacklist/refresh
func (h *Handler) RefreshBlacklistCache(w http.ResponseWriter, r *http.Request) {
	if err := h.auth.RefreshBlacklistCache(); err != nil {
		http.Error(w, "Failed to refresh blacklist cache", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Blacklist cache refreshed successfully"))
}

// GetBlacklistedUsers handles GET /v1/admin/blacklist
func (h *Handler) GetBlacklistedUsers(w http.ResponseWriter, r *http.Request) {
	entries, err := h.auth.GetBlacklistedUsersWithDetails()
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	// If no entries, return empty array instead of null
	if entries == nil {
		entries = []domain.BlacklistEntry{}
	}

	writeJSON(w, api.BlacklistResponse{Users: entries})
}
