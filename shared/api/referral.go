package api

import "github.com/itchan-dev/itchan/shared/domain"

// Request DTOs

type RecordReferralVisitRequest struct {
	Source string `json:"source" validate:"required"`
}

// Response DTOs

type ReferralStatsResponse struct {
	Stats []domain.ReferralStats `json:"stats"`
}
