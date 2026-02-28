package handler

import "net/http"

func (h *Handler) WelcomeGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, r, "welcome.html", nil)
}
