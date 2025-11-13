package handler

import (
	"net/http"
)

func (h *Handler) ProposalHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "proposal.html", nil)
}
