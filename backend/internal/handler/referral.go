package handler

import (
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/utils"
)

// RecordReferralVisit handles POST /v1/referral/visit
func (h *Handler) RecordReferralVisit(w http.ResponseWriter, r *http.Request) {
	var req api.RecordReferralVisitRequest
	if err := utils.DecodeValidate(r.Body, &req); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	if err := h.referral.RecordVisit(req.Source); err != nil {
		http.Error(w, "Failed to record visit", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetReferralStats handles GET /v1/admin/referral/stats
func (h *Handler) GetReferralStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.referral.GetStats()
	if err != nil {
		http.Error(w, "Failed to get referral stats", http.StatusInternalServerError)
		return
	}

	if stats == nil {
		stats = []domain.ReferralStats{}
	}

	writeJSON(w, stats)
}
