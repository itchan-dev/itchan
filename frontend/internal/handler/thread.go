package handler

import (
	"github.com/itchan-dev/itchan/shared/logger"
	"fmt"
	"html/template"

	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

func (h *Handler) ThreadGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Thread     *frontend_domain.Thread
		Error      template.HTML
		User       *domain.User
		Validation ValidationData
	}
	templateData.User = mw.GetUserFromContext(r)
	vars := mux.Vars(r)
	shortName := vars["board"]
	threadId := vars["thread"]
	templateData.Error, _ = parseMessagesFromQuery(r)

	thread, err := h.APIClient.GetThread(r, shortName, threadId)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}

	templateData.Thread = RenderThread(thread)
	templateData.Validation = h.NewValidationData()

	h.renderTemplate(w, "thread.html", templateData)
}

func (h *Handler) ThreadPostHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortName := vars["board"]
	threadIdStr := vars["thread"]

	// Preserve the anchor for both success and error redirects
	targetURL := fmt.Sprintf("/%s/%s#bottom", shortName, threadIdStr)
	errorTargetURL := fmt.Sprintf("/%s/%s#top", shortName, threadIdStr)

	threadId, err := strconv.Atoi(threadIdStr)
	if err != nil {
		redirectWithParams(w, r, errorTargetURL, map[string]string{"error": "Invalid thread ID."})
		return
	}

	// Validate request size and parse multipart form
	if !h.parseAndValidateMultipartForm(w, r, errorTargetURL) {
		return
	}

	text := r.FormValue("text")
	processedText, domainReplies, hasPayload := h.processMessageText(text, domain.MessageMetadata{
		Board:    shortName,
		ThreadId: domain.ThreadId(threadId),
	})

	// Check if message has either text OR attachments (align with backend validation)
	hasAttachments := r.MultipartForm != nil && r.MultipartForm.File != nil && len(r.MultipartForm.File["attachments"]) > 0
	if !hasPayload && !hasAttachments {
		redirectWithParams(w, r, errorTargetURL, map[string]string{"error": "Message must contain either text or attachments."})
		return
	}

	backendData := api.CreateMessageRequest{
		Text:    processedText,
		ReplyTo: domainReplies,
	}

	err = h.APIClient.CreateReply(r, shortName, threadIdStr, backendData, r.MultipartForm)
	if err != nil {
		logger.Log.Error("posting reply via API", "error", err)
		redirectWithParams(w, r, errorTargetURL, map[string]string{"error": template.HTMLEscapeString(err.Error())})
		return
	}

	// Success, redirect back to the thread, which will show the new message
	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}

func (h *Handler) ThreadDeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boardShortName := vars["board"]
	threadId := vars["thread"]
	targetURL := "/" + boardShortName // Redirect to board page

	err := h.APIClient.DeleteThread(r, boardShortName, threadId)
	if err != nil {
		logger.Log.Error("deleting thread via API", "error", err)
		redirectWithParams(w, r, targetURL, map[string]string{"error": template.HTMLEscapeString(err.Error())})
		return
	}

	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}
