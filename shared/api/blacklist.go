package api

import "github.com/itchan-dev/itchan/shared/domain"

// Request DTOs

type BlacklistUserRequest struct {
	Reason string `json:"reason"`
}

// Response DTOs

type BlacklistResponse struct {
	Users []domain.BlacklistEntry `json:"users"`
}
