package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
	"github.com/itchan-dev/itchan/shared/utils"
)

func (h *Handler) BoardGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
		Board       *frontend_domain.Board
		CurrentPage int
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)
	shortName := chi.URLParam(r, "board")

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if pageInt, err := strconv.Atoi(pageStr); err == nil && pageInt > 0 {
			page = pageInt
		}
	}
	templateData.CurrentPage = page

	board, err := h.APIClient.GetBoard(r, shortName, page)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err) // Renders a dedicated error page
		return
	}

	if checkNotModified(w, r, board.LastActivityAt) {
		return
	}

	templateData.Board = RenderBoard(board)

	h.renderTemplate(w, "board.html", templateData)
}

func (h *Handler) BoardPostHandler(w http.ResponseWriter, r *http.Request) {
	shortName := chi.URLParam(r, "board")
	errorTargetURL := "/" + shortName

	// Validate request size and parse multipart form
	if !h.parseAndValidateMultipartForm(w, r, errorTargetURL) {
		return
	}

	text := r.FormValue("text")
	processedText, domainReplies, hasPayload, err := h.processMessageText(text, domain.MessageMetadata{Board: shortName})
	if err != nil {
		logger.Log.Error("processing message text", "error", err)
		h.redirectWithFlash(w, r, errorTargetURL, flashCookieError, err.Error())
		return
	}

	// Check if message has either text OR attachments (align with backend validation)
	hasAttachments := r.MultipartForm != nil && r.MultipartForm.File != nil && len(r.MultipartForm.File["attachments"]) > 0
	if !hasPayload && !hasAttachments {
		h.redirectWithFlash(w, r, errorTargetURL, flashCookieError, "Message must contain either text or attachments.")
		return
	}

	backendData := api.CreateThreadRequest{
		Title: r.FormValue("title"),
		OpMessage: api.CreateMessageRequest{
			Text:            processedText,
			ShowEmailDomain: r.FormValue("show_company") == "on",
			ReplyTo:         domainReplies,
		},
	}

	newThreadID, err := h.APIClient.CreateThread(r, shortName, backendData, r.MultipartForm)
	if err != nil {
		logger.Log.Error("creating thread via API", "error", err)
		h.redirectWithFlash(w, r, errorTargetURL, flashCookieError, err.Error())
		return
	}

	successTargetURL := "/" + shortName + "/" + newThreadID
	http.Redirect(w, r, successTargetURL, http.StatusSeeOther)
}

func (h *Handler) BoardDeleteHandler(w http.ResponseWriter, r *http.Request) {
	shortName := chi.URLParam(r, "board")
	targetURL := "/" // Redirect to index page

	err := h.APIClient.DeleteBoard(r, shortName)
	if err != nil {
		logger.Log.Error("deleting board via API", "error", err)
		h.redirectWithFlash(w, r, targetURL, flashCookieError, err.Error())
		return
	}

	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}
