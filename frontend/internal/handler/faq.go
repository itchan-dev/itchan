package handler

import "net/http"

func (h *Handler) FAQGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, r, "faq.html", nil)
}
