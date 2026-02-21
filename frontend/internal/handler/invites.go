package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
	mw "github.com/itchan-dev/itchan/shared/middleware"
)

// InvitesGetHandler displays the invites page with user's invite codes
func (h *Handler) InvitesGetHandler(w http.ResponseWriter, r *http.Request) {
	user := mw.GetUserFromContext(r)

	var templateData struct {
		Invites          []domain.InviteCode
		RemainingInvites int
		AccountAgeDays   int
		RequiredAgeDays  int
		IsEligibleByAge  bool
	}

	invites, err := h.APIClient.GetMyInvites(r)
	var errMsg string
	if err != nil {
		logger.Log.Error("failed to get invites from API", "error", err)
		errMsg = fmt.Sprintf("Failed to load invites: %v", err)
		invites = []domain.InviteCode{}
	}
	templateData.Invites = invites

	if user.Admin {
		templateData.RemainingInvites = -1
	} else {
		unusedCount := 0
		for _, invite := range invites {
			if invite.UsedBy == nil {
				unusedCount++
			}
		}
		maxInvites := h.Public.MaxInvitesPerUser
		templateData.RemainingInvites = max(maxInvites-unusedCount, 0)
	}

	accountAge := time.Since(user.CreatedAt)
	templateData.AccountAgeDays = int(accountAge.Hours() / 24)

	requiredAge := h.Public.MinAccountAgeForInvites
	templateData.RequiredAgeDays = int(requiredAge.Hours() / 24)

	templateData.IsEligibleByAge = user.Admin || accountAge >= requiredAge

	h.renderTemplateWithError(w, r, "invites.html", templateData, errMsg)
}

// GenerateInvitePostHandler generates a new invite code
func (h *Handler) GenerateInvitePostHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		logger.Log.Error("parsing form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Call API to generate invite
	invite, err := h.APIClient.GenerateInvite(r)
	if err != nil {
		logger.Log.Error("generating invite via API", "error", err)
		h.redirectWithFlash(w, r, "/invites", flashCookieError, err.Error())
		return
	}

	successMsg := fmt.Sprintf("Invite code generated: %s (save this now, it won't be shown again)", invite.PlainCode)
	h.redirectWithFlash(w, r, "/invites", flashCookieSuccess, successMsg)
}

// RevokeInvitePostHandler revokes (deletes) an unused invite code
func (h *Handler) RevokeInvitePostHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		logger.Log.Error("parsing form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get codeHash from form
	codeHash := r.FormValue("codeHash")
	if codeHash == "" {
		http.Error(w, "Missing codeHash", http.StatusBadRequest)
		return
	}

	// Call API to revoke invite
	err := h.APIClient.RevokeInvite(r, codeHash)
	if err != nil {
		logger.Log.Error("revoking invite via API", "error", err)
		h.redirectWithFlash(w, r, "/invites", flashCookieError, err.Error())
		return
	}

	h.redirectWithFlash(w, r, "/invites", flashCookieSuccess, "Invite revoked successfully")
}
