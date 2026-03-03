package handler

import (
	"net/http"

	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
	mw "github.com/itchan-dev/itchan/shared/middleware"
)

func domainAllowed(emailDomain string, allowedDomains []string) bool {
	for _, d := range allowedDomains {
		if d == emailDomain {
			return true
		}
	}
	return false
}

func (h *Handler) IndexGetHandler(w http.ResponseWriter, r *http.Request) {
	boards, err := h.APIClient.GetBoards(r)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}

	user := mw.GetUserFromContext(r)

	var pageData frontend_domain.IndexPageData
	for _, b := range boards {
		if len(b.AllowedEmailDomains) > 0 {
			accessible := user != nil && (user.Admin || domainAllowed(user.EmailDomain, b.AllowedEmailDomains))
			pageData.CorporateBoards = append(pageData.CorporateBoards, frontend_domain.BoardWithAccess{Board: b, Accessible: accessible})
		} else {
			pageData.PublicBoards = append(pageData.PublicBoards, b)
		}
	}

	h.renderTemplateWithError(w, r, "index.html", pageData, errMsg)
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
		h.redirectWithFlash(w, r, targetURL, flashCookieError, err.Error())
		return
	}

	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}
