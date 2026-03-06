package handler

import (
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/utils"
)

// RecordReferralAction handles POST /v1/auth/referral/action
func (h *Handler) RecordReferralAction(w http.ResponseWriter, r *http.Request) {
	var req api.RecordReferralActionRequest
	if err := utils.DecodeValidate(r.Body, &req); err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	ip, _ := utils.GetIP(r)
	if err := h.referral.RecordAction(req.Source, req.Action, ip); err != nil {
		http.Error(w, "Failed to record action", http.StatusInternalServerError)
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
		stats = []domain.ReferralActionStats{}
	}

	writeJSON(w, stats)
}
