package handler

import "net/http"

func (h *Handler) AboutGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, r, "about.html", nil)
}

func (h *Handler) TermsGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, r, "terms.html", nil)
}

func (h *Handler) PrivacyGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, r, "privacy.html", nil)
}

func (h *Handler) ContactsGetHandler(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, r, "contacts.html", nil)
}
