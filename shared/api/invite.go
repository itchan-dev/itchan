package api

import "github.com/itchan-dev/itchan/shared/domain"

// Response DTOs

type InviteListResponse struct {
	Invites []domain.InviteCode `json:"invites"`
	Page    int                 `json:"page"`
}
