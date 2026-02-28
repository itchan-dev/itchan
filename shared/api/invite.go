package api

import (
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
)

// Response DTOs

type InviteListResponse struct {
	Invites []domain.InviteCode `json:"invites"`
	Page    int                 `json:"page"`
}

type GenerateInviteResponse struct {
	InviteCode string    `json:"invite_code"`
	ExpiresAt  time.Time `json:"expires_at"`
}
