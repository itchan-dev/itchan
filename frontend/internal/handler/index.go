package handler

import (
	"html/template"
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
)

func (h *Handler) IndexGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
		Boards []domain.Board
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)

	boards, err := h.APIClient.GetBoards(r)
	if err != nil {
		templateData.Error = template.HTML(template.HTMLEscapeString(err.Error()))
	}
	templateData.Boards = boards

	h.renderTemplate(w, "index.html", templateData)
}

func (h *Handler) IndexPostHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := "/"

	if err := r.ParseForm(); err != nil {
		h.redirectWithFlash(w, r, targetURL, flashCookieError, "Invalid form data.")
		return
	}

	shortName := r.FormValue("shortName")
	name := r.FormValue("name")
	allowedEmailsStr := r.FormValue("allowedEmails")

	backendData := api.CreateBoardRequest{
		Name:      name,
		ShortName: shortName,
	}

	if allowedEmailsStr != "" {
		allowedEmails := domain.Emails(splitAndTrim(allowedEmailsStr))
		if len(allowedEmails) > 0 {
			backendData.AllowedEmails = &allowedEmails
		}
	}

	err := h.APIClient.CreateBoard(r, backendData)
	if err != nil {
		logger.Log.Error("creating board via API", "error", err)
		h.redirectWithFlash(w, r, targetURL, flashCookieError, template.HTMLEscapeString(err.Error()))
		return
	}

	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}
