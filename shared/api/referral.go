package api

// Request DTOs

type RecordReferralActionRequest struct {
	Source string `json:"source" validate:"required"`
	Action string `json:"action" validate:"required"`
}
