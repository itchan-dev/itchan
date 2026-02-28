package api

// Request DTOs

type RecordReferralVisitRequest struct {
	Source string `json:"source" validate:"required"`
}
