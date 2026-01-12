package handler

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

// RegisterWithInvite handles POST /v1/auth/register_with_invite
// Allows users to register using an invite code instead of email confirmation
func (h *Handler) RegisterWithInvite(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		InviteCode string `json:"invite_code" validate:"required"`
		Password   string `json:"password" validate:"required"`
	}

	if err := utils.DecodeValidate(r.Body, &reqBody); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	// Register and get generated email
	email, err := h.auth.RegisterWithInvite(reqBody.InviteCode, domain.Password(reqBody.Password))
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	writeJSON(w, map[string]string{
		"message": "Registration successful. You can now log in.",
		"email":   email,
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

	writeJSON(w, map[string]any{
		"invite_code": invite.PlainCode,
		"expires_at":  invite.ExpiresAt.Format(time.RFC3339),
	})
}

// GetMyInvites handles GET /v1/invites
// Returns all invite codes created by the authenticated user
func (h *Handler) GetMyInvites(w http.ResponseWriter, r *http.Request) {
	user := mw.GetUserFromContext(r)

	invites, err := h.auth.GetUserInvites(user.Id)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	// Return empty array instead of null if no invites
	if invites == nil {
		invites = []domain.InviteCode{}
	}

	writeJSON(w, invites)
}

// RevokeInvite handles DELETE /v1/invites/{codeHash}
// Deletes an unused invite code owned by the authenticated user
func (h *Handler) RevokeInvite(w http.ResponseWriter, r *http.Request) {
	user := mw.GetUserFromContext(r)
	vars := mux.Vars(r)
	codeHash := vars["codeHash"]

	if err := h.auth.RevokeInvite(user.Id, codeHash); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Invite revoked successfully"))
}
