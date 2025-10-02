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

	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

func getBoards(r *http.Request) ([]domain.Board, error) {
	req, err := requestWithCookie(r, "GET", "http://api:8080/v1/boards", nil, "accessToken")
	if err != nil {
		return nil, errors.New("internal error: request creation failed")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error getting boards from API: %v", err)
		return nil, errors.New("internal error: backend unavailable")
	}
	defer resp.Body.Close() // Ensure body is closed

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body) // Read body for context even on error
		log.Printf("API backend error (%d): %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("internal error: backend returned status %d", resp.StatusCode)
	}

	var boards []domain.Board
	if err := utils.Decode(resp.Body, &boards); err != nil {
		log.Printf("Error decoding boards response: %v", err)
		return nil, errors.New("internal error: cannot decode response")
	}
	return boards, nil
}

func (h *Handler) IndexGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Boards     []domain.Board
		Error      template.HTML
		User       *domain.User
		Validation struct {
			BoardNameMaxLen      int
			BoardShortNameMaxLen int
		}
	}
	templateData.User = mw.GetUserFromContext(r)
	templateData.Error, _ = parseMessagesFromQuery(r) // Get error from query param

	boards, err := getBoards(r)
	if err != nil {
		// If getting boards fails, display that error instead of query param error
		templateData.Error = template.HTML(template.HTMLEscapeString(err.Error()))
		// Still render the page, but likely without boards list
		h.renderTemplate(w, "index.html", templateData)
		return
	}
	templateData.Boards = boards
	templateData.Validation.BoardNameMaxLen = h.Public.BoardNameMaxLen
	templateData.Validation.BoardShortNameMaxLen = h.Public.BoardShortNameMaxLen

	h.renderTemplate(w, "index.html", templateData)
}

// POST handler for creating a new board on the index page
func (h *Handler) IndexPostHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := "/" // Redirect target on success or error

	// Parse form data
	shortName := r.FormValue("shortName")
	name := r.FormValue("name")
	var allowedEmails []string
	if allowedEmailsStr := r.FormValue("allowedEmails"); allowedEmailsStr != "" {
		allowedEmails = splitAndTrim(allowedEmailsStr)
	}

	// Prepare backend request data
	backendData := struct {
		Name          string    `json:"name"`
		ShortName     string    `json:"short_name"`
		AllowedEmails *[]string `json:"allowed_emails,omitempty"`
	}{Name: name, ShortName: shortName}
	// Only include allowedEmails if it's not empty
	if len(allowedEmails) > 0 {
		backendData.AllowedEmails = &allowedEmails
	}

	backendDataJson, err := json.Marshal(backendData)
	if err != nil {
		log.Printf("Error marshalling board data: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: could not prepare data.")
		return
	}

	// Create request with authentication
	req, err := requestWithCookie(r, "POST", "http://api:8080/v1/admin/boards", bytes.NewBuffer(backendDataJson), "accessToken")
	if err != nil {
		log.Printf("Error creating request for board creation: %v", err)
		// requestWithCookie might return sensitive info, use generic message
		redirectWithError(w, r, targetURL, "Internal error: could not create request.")
		return
	}

	// Send request to backend
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error contacting backend API for board creation: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: backend unavailable.")
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Error creating board (status %d)", resp.StatusCode) // Default message
		if readErr == nil && len(bodyBytes) > 0 {
			// Use backend error message if available and readable
			errMsg = string(bodyBytes)
		} else if readErr != nil {
			log.Printf("Error reading backend error response body: %v", readErr)
		}
		redirectWithError(w, r, targetURL, errMsg)
		return
	}

	// Success: Redirect back to the index page (GET handler)
	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}
