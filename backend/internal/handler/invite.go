package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

// RegisterWithInvite handles POST /v1/auth/register_with_invite
// Allows users to register using an invite code instead of email confirmation
func (h *Handler) RegisterWithInvite(w http.ResponseWriter, r *http.Request) {
	var req api.RegisterWithInviteRequest

	if err := utils.DecodeValidate(r.Body, &req); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	// Register and get generated email
	email, err := h.auth.RegisterWithInvite(req.InviteCode, domain.Password(req.Password), req.RefSource)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	writeJSON(w, api.RegisterWithInviteResponse{
		Message: "Registration successful. You can now log in.",
		Email:   email,
	})
}

// GenerateInvite handles POST /v1/invites
// Creates a new invite code for the authenticated user
func (h *Handler) GenerateInvite(w http.ResponseWriter, r *http.Request) {
	user := mw.GetUserFromContext(r)

	invite, err := h.auth.GenerateInvite(*user)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	writeJSON(w, api.GenerateInviteResponse{
		InviteCode: invite.PlainCode,
		ExpiresAt:  invite.ExpiresAt,
	})
}

// GetMyInvites handles GET /v1/invites
// Returns invite codes created by the authenticated user, with pagination.
func (h *Handler) GetMyInvites(w http.ResponseWriter, r *http.Request) {
	user := mw.GetUserFromContext(r)

	page := utils.GetPage(r)

	invites, err := h.auth.GetUserInvites(user.Id, page)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	// Return empty array instead of null if no invites
	if invites == nil {
		invites = []domain.InviteCode{}
	}

	writeJSON(w, api.InviteListResponse{Invites: invites, Page: page})
}

// RevokeInvite handles DELETE /v1/invites/{codeHash}
// Deletes an unused invite code owned by the authenticated user
func (h *Handler) RevokeInvite(w http.ResponseWriter, r *http.Request) {
	user := mw.GetUserFromContext(r)
	codeHash := chi.URLParam(r, "codeHash")

	if err := h.auth.RevokeInvite(user.Id, codeHash); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
