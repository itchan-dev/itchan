package domain

// ReferralStats contains aggregated visit and registration counts per source.
type ReferralStats struct {
	Source         string  `json:"source"`
	VisitCount     int     `json:"visit_count"`
	RegisterCount  int     `json:"register_count"`
	ConversionRate float64 `json:"conversion_rate"`
}
