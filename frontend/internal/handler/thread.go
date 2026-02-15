package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
	"github.com/itchan-dev/itchan/shared/utils"
)

func (h *Handler) ThreadGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		CommonTemplateData
		Thread *frontend_domain.Thread
	}
	templateData.CommonTemplateData = h.InitCommonTemplateData(w, r)
	shortName := chi.URLParam(r, "board")
	threadId := chi.URLParam(r, "thread")

	// Parse page parameter (default to 1)
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if parsedPage, err := strconv.Atoi(pageStr); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	thread, err := h.APIClient.GetThread(r, shortName, threadId, page)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	if checkNotModified(w, r, thread.LastModifiedAt) {
		return
	}

	templateData.Thread = RenderThread(thread)

	h.renderTemplate(w, "thread.html", templateData)
}

func (h *Handler) ThreadPostHandler(w http.ResponseWriter, r *http.Request) {
	shortName := chi.URLParam(r, "board")
	threadIdStr := chi.URLParam(r, "thread")

	errorTargetURL := fmt.Sprintf("/%s/%s#top", shortName, threadIdStr)

	threadId, err := strconv.Atoi(threadIdStr)
	if err != nil {
		h.redirectWithFlash(w, r, errorTargetURL, flashCookieError, "Invalid thread ID.")
		return
	}

	// Validate request size and parse multipart form
	if !h.parseAndValidateMultipartForm(w, r, errorTargetURL) {
		return
	}

	text := r.FormValue("text")
	processedText, domainReplies, hasPayload, err := h.processMessageText(text, domain.MessageMetadata{
		Board:    shortName,
		ThreadId: domain.ThreadId(threadId),
	})
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

	backendData := api.CreateMessageRequest{
		Text:            processedText,
		ShowEmailDomain: r.FormValue("show_company") == "on",
		ReplyTo:         domainReplies,
	}

	page, err := h.APIClient.CreateReply(r, shortName, threadIdStr, backendData, r.MultipartForm)
	if err != nil {
		logger.Log.Error("posting reply via API", "error", err)
		h.redirectWithFlash(w, r, errorTargetURL, flashCookieError, err.Error())
		return
	}

	targetURL := fmt.Sprintf("/%s/%s?page=%d#bottom", shortName, threadIdStr, page)
	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}

func (h *Handler) ThreadDeleteHandler(w http.ResponseWriter, r *http.Request) {
	boardShortName := chi.URLParam(r, "board")
	threadId := chi.URLParam(r, "thread")
	targetURL := "/" + boardShortName // Redirect to board page

	err := h.APIClient.DeleteThread(r, boardShortName, threadId)
	if err != nil {
		logger.Log.Error("deleting thread via API", "error", err)
		h.redirectWithFlash(w, r, targetURL, flashCookieError, err.Error())
		return
	}

	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}

func (h *Handler) ThreadTogglePinnedHandler(w http.ResponseWriter, r *http.Request) {
	boardShortName := chi.URLParam(r, "board")
	threadId := chi.URLParam(r, "thread")

	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = fmt.Sprintf("/%s/%s", boardShortName, threadId)
	}

	_, err := h.APIClient.TogglePinnedThread(r, boardShortName, threadId)
	if err != nil {
		logger.Log.Error("toggling pin via API", "error", err)
		h.redirectWithFlash(w, r, referer, flashCookieError, err.Error())
		return
	}

	http.Redirect(w, r, referer, http.StatusSeeOther)
}
