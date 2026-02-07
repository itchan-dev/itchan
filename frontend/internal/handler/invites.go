package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
)

// InvitesGetHandler displays the invites page with user's invite codes
func (h *Handler) InvitesGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
		Invites          []domain.InviteCode
		RemainingInvites int
		AccountAgeDays   int
		RequiredAgeDays  int
		IsEligibleByAge  bool
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)

	// Fetch user's invites from API
	invites, err := h.APIClient.GetMyInvites(r)
	if err != nil {
		logger.Log.Error("failed to get invites from API", "error", err)
		templateData.Error = template.HTML(template.HTMLEscapeString(fmt.Sprintf("Failed to load invites: %v", err)))
		invites = []domain.InviteCode{} // Use empty array on error
	}
	templateData.Invites = invites

	// Calculate remaining invites
	if templateData.User.Admin {
		// Admins have unlimited invites (represented as -1)
		templateData.RemainingInvites = -1
	} else {
		// Count unused invites (where UsedBy is nil)
		unusedCount := 0
		for _, invite := range invites {
			if invite.UsedBy == nil {
				unusedCount++
			}
		}

		// Calculate remaining invites
		maxInvites := h.Public.MaxInvitesPerUser
		templateData.RemainingInvites = max(maxInvites-unusedCount, 0)
	}

	// Calculate account age
	accountAge := time.Since(templateData.User.CreatedAt)
	templateData.AccountAgeDays = int(accountAge.Hours() / 24)

	// Get required age from config (in days)
	requiredAge := h.Public.MinAccountAgeForInvites
	templateData.RequiredAgeDays = int(requiredAge.Hours() / 24)

	// Check if user is eligible by age
	templateData.IsEligibleByAge = templateData.User.Admin || accountAge >= requiredAge

	h.renderTemplate(w, "invites.html", templateData)
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
		h.redirectWithFlash(w, r, "/invites", flashCookieError, template.HTMLEscapeString(err.Error()))
		return
	}

	successMsg := fmt.Sprintf("Invite code generated: %s (save this now, it won't be shown again)", template.HTMLEscapeString(invite.PlainCode))
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
		h.redirectWithFlash(w, r, "/invites", flashCookieError, template.HTMLEscapeString(err.Error()))
		return
	}

	h.redirectWithFlash(w, r, "/invites", flashCookieSuccess, "Invite revoked successfully")
}
