package handler

import "net/http"

func (h *Handler) FAQGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)
	h.renderTemplate(w, "faq.html", templateData)
}
