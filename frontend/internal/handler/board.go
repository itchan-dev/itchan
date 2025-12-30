package handler

import (
	"github.com/itchan-dev/itchan/shared/logger"
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

func (h *Handler) BoardGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Board       *frontend_domain.Board
		Error       template.HTML
		User        *domain.User
		CurrentPage int
		Validation  ValidationData
	}
	templateData.User = mw.GetUserFromContext(r)
	shortName := mux.Vars(r)["board"]
	templateData.Error, _ = parseMessagesFromQuery(r)

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

	templateData.Board = RenderBoard(board)
	templateData.Validation = h.NewValidationData()

	h.renderTemplate(w, "board.html", templateData)
}

func (h *Handler) BoardPostHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortName := vars["board"]
	errorTargetURL := "/" + shortName

	// Validate request size and parse multipart form
	if !h.parseAndValidateMultipartForm(w, r, errorTargetURL) {
		return
	}

	text := r.FormValue("text")
	processedText, domainReplies, hasPayload := h.processMessageText(text, domain.MessageMetadata{Board: shortName})

	if !hasPayload {
		redirectWithParams(w, r, errorTargetURL, map[string]string{"error": "Message has empty payload."})
		return
	}

	backendData := api.CreateThreadRequest{
		Title: r.FormValue("title"),
		OpMessage: api.CreateMessageRequest{
			Text:    processedText,
			ReplyTo: domainReplies,
		},
	}

	newThreadID, err := h.APIClient.CreateThread(r, shortName, backendData, r.MultipartForm)
	if err != nil {
		logger.Log.Error("creating thread via API", "error", err)
		redirectWithParams(w, r, errorTargetURL, map[string]string{"error": err.Error()})
		return
	}

	successTargetURL := "/" + shortName + "/" + newThreadID
	http.Redirect(w, r, successTargetURL, http.StatusSeeOther)
}

func (h *Handler) BoardDeleteHandler(w http.ResponseWriter, r *http.Request) {
	shortName := mux.Vars(r)["board"]
	targetURL := "/" // Redirect to index page

	err := h.APIClient.DeleteBoard(r, shortName)
	if err != nil {
		logger.Log.Error("deleting board via API", "error", err)
		redirectWithParams(w, r, targetURL, map[string]string{"error": err.Error()})
		return
	}

	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}
