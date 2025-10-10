package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

// DELETE handler uses redirects already, no change needed here per se,
// but ensure it uses redirectWithError for consistency.
func BoardDeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortName := vars["board"]
	targetURL := "/" // Redirect target

	req, err := requestWithCookie(r, "DELETE", fmt.Sprintf("http://api:8080/v1/admin/%s", shortName), nil, "accessToken")
	if err != nil {
		// Use helper for consistency
		redirectWithError(w, r, targetURL, fmt.Sprintf("Internal error: %s", err.Error()))
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error contacting backend API for board deletion: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: backend unavailable.")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Error deleting board (status %d)", resp.StatusCode)
		if readErr == nil && len(bodyBytes) > 0 {
			errMsg = string(bodyBytes)
		} else if readErr != nil {
			log.Printf("Error reading backend error response body: %v", readErr)
		}
		redirectWithError(w, r, targetURL, errMsg)
		return
	}

	// Success: Redirect back to the index page
	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}

func getBoardPreview(r *http.Request, shortName string, page int) (domain.Board, error) {
	// Construct URL carefully, handling empty page param
	apiURL := fmt.Sprintf("http://api:8080/v1/%s", shortName)
	if page > 1 {
		apiURL = fmt.Sprintf("%s?page=%d", apiURL, page)
	}

	req, err := requestWithCookie(r, "GET", apiURL, nil, "accessToken")
	if err != nil {
		return domain.Board{}, errors.New("internal error: request creation failed")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error getting board preview from API: %v", err)
		return domain.Board{}, errors.New("internal error: backend unavailable")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return domain.Board{}, &internal_errors.ErrorWithStatusCode{Message: fmt.Sprintf("board /%s not found", shortName), StatusCode: http.StatusNotFound}
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("API backend error (%d) getting board preview: %s", resp.StatusCode, string(bodyBytes))
		return domain.Board{}, fmt.Errorf("internal error: backend returned status %d", resp.StatusCode)
	}

	var board domain.Board
	if err := utils.Decode(resp.Body, &board); err != nil {
		return domain.Board{}, errors.New("internal error: cannot decode response")
	}
	return board, nil
}

func (h *Handler) BoardGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Board       *frontend_domain.Board
		Error       template.HTML
		User        *domain.User
		CurrentPage int
		Validation  struct {
			ThreadTitleMaxLen int
			MessageTextMaxLen int
		}
	}
	templateData.User = mw.GetUserFromContext(r)
	shortName := mux.Vars(r)["board"]
	templateData.Error, _ = parseMessagesFromQuery(r) // Get error from query param

	pageStr := r.URL.Query().Get("page")
	page := 1 // Default to page 1
	if pageStr != "" {
		pageInt, err := strconv.Atoi(pageStr)
		if err == nil && pageInt > 0 {
			page = pageInt
		}
	}
	templateData.CurrentPage = page

	board, err := getBoardPreview(r, shortName, page)
	if err != nil {
		utils.WriteErrorAndStatusCode(w, err)
		return
	}
	boardRendered := RenderBoard(board)
	templateData.Board = boardRendered
	templateData.Validation.ThreadTitleMaxLen = h.Public.ThreadTitleMaxLen
	templateData.Validation.MessageTextMaxLen = h.Public.MessageTextMaxLen

	h.renderTemplate(w, "board.html", templateData)
}

// POST handler for creating a new thread on a board page
func (h *Handler) BoardPostHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortName := vars["board"]

	// Redirect target on error (back to the board page)
	errorTargetURL := fmt.Sprintf("/%s", shortName)

	// Parse form data (assuming title, text, maybe attachments)
	title := r.FormValue("title")
	text := r.FormValue("text")
	// TODO: Handle attachments if necessary (multipart form?)
	processedText, replyTo := h.TextProcessor.ProcessMessage(domain.Message{Text: text, MessageMetadata: domain.MessageMetadata{Board: shortName}})
	// TODO: Handle attachments if necessary

	// Prepare backend request data using shared API DTOs
	var domainReplies domain.Replies
	for _, r := range replyTo {
		if r != nil {
			domainReplies = append(domainReplies, &r.Reply)
		}
	}
	msgData := api.CreateMessageRequest{
		Text:    processedText,
		ReplyTo: &domainReplies,
	}
	// Prepare backend request data using shared API DTOs
	backendData := api.CreateThreadRequest{
		Title:     title,
		OpMessage: msgData,
	}

	backendDataJson, err := json.Marshal(backendData)
	if err != nil {
		log.Printf("Error marshalling new thread data: %v", err)
		redirectWithError(w, r, errorTargetURL, "Internal error: could not prepare data.")
		return
	}

	// Create request with authentication
	apiEndpoint := fmt.Sprintf("http://api:8080/v1/%s", shortName)
	req, err := requestWithCookie(r, "POST", apiEndpoint, bytes.NewBuffer(backendDataJson), "accessToken")
	if err != nil {
		log.Printf("Error creating request for thread creation: %v", err)
		redirectWithError(w, r, errorTargetURL, "Internal error: could not create request.")
		return
	}

	// Send request to backend
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error contacting backend API for thread creation: %v", err)
		redirectWithError(w, r, errorTargetURL, "Internal error: backend unavailable.")
		return
	}
	defer resp.Body.Close()

	// Read response body regardless of status code
	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		log.Printf("Error reading backend response body after thread creation attempt: %v", readErr)
		// Redirect with a generic error if body cannot be read
		redirectWithError(w, r, errorTargetURL, fmt.Sprintf("Backend status %d, but response unreadable.", resp.StatusCode))
		return
	}
	msg := string(bodyBytes) // This should be the new thread ID on success

	// Check response status
	if resp.StatusCode != http.StatusCreated {
		errMsg := fmt.Sprintf("Error creating thread (status %d)", resp.StatusCode) // Default
		if len(msg) > 0 {
			errMsg = fmt.Sprintf("Backend error: %s", msg) // Use backend message if available
		}
		redirectWithError(w, r, errorTargetURL, errMsg)
		return
	}

	// Success: Redirect to the newly created thread page
	// The response body 'msg' is expected to be the new thread ID
	successTargetURL := fmt.Sprintf("/%s/%s", shortName, msg)
	http.Redirect(w, r, successTargetURL, http.StatusSeeOther)
}
