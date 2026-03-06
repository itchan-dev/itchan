package domain

// ReferralActionStats contains aggregated counts per source and action type.
type ReferralActionStats struct {
	Source string `json:"source"`
	Action string `json:"action"`
	Count  int    `json:"count"`
}
