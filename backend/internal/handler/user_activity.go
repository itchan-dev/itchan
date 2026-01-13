package handler

import (
	"net/http"

	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

// GetUserActivity returns the authenticated user's recent messages
func (h *Handler) GetUserActivity(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user := mw.GetUserFromContext(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Fetch user activity from service
	activity, err := h.userActivity.GetUserActivity(user.Id)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	// Return JSON response
	writeJSON(w, activity)
}
