package handler

import "net/http"

func (h *Handler) AboutGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)
	h.renderTemplate(w, "about.html", templateData)
}

func (h *Handler) TermsGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)
	h.renderTemplate(w, "terms.html", templateData)
}

func (h *Handler) PrivacyGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)
	h.renderTemplate(w, "privacy.html", templateData)
}

func (h *Handler) ContactsGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)
	h.renderTemplate(w, "contacts.html", templateData)
}
