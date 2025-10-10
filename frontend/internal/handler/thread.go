package handler

import (
	"bytes"
	"encoding/json"
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
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

func (h *Handler) ThreadGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Thread     *frontend_domain.Thread
		Error      template.HTML
		User       *domain.User
		Validation struct {
			MessageTextMaxLen int
		}
	}
	templateData.User = mw.GetUserFromContext(r)
	vars := mux.Vars(r)
	shortName := vars["board"]
	threadId := vars["thread"]
	templateData.Error, _ = parseMessagesFromQuery(r) // Get error from query param

	// Fetch thread data from backend
	apiURL := fmt.Sprintf("http://api:8080/v1/%s/%s", shortName, threadId)
	req, err := requestWithCookie(r, "GET", apiURL, nil, "accessToken")
	if err != nil {
		http.Error(w, "Internal error: could not create request", http.StatusInternalServerError)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error getting thread data from API: %v", err)
		http.Error(w, "Internal error: backend unavailable", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		http.Error(w, fmt.Sprintf("Thread /%s/%s not found", shortName, threadId), http.StatusNotFound)
		return
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("API backend error (%d) getting thread: %s", resp.StatusCode, string(bodyBytes))
		http.Error(w, fmt.Sprintf("Internal error: backend returned status %d", resp.StatusCode), http.StatusInternalServerError)
		return
	}

	var thread domain.Thread
	if err := utils.Decode(resp.Body, &thread); err != nil {
		log.Printf("Error decoding thread response: %v", err)
		http.Error(w, "Internal error: cannot decode response", http.StatusInternalServerError)
		return
	}
	threadRendered := RenderThread(thread)
	templateData.Thread = threadRendered
	templateData.Validation.MessageTextMaxLen = h.Public.MessageTextMaxLen

	h.renderTemplate(w, "thread.html", templateData)
}

// POST handler for creating a new reply within a thread
func (h *Handler) ThreadPostHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortName := vars["board"]
	threadIdStr := vars["thread"]

	// Redirect target (back to the same thread page on success or error)
	targetURL := fmt.Sprintf("/%s/%s#bottom", shortName, threadIdStr)
	errorTargetURL := fmt.Sprintf("/%s/%s", shortName, threadIdStr)

	threadId, err := strconv.Atoi(threadIdStr)
	if err != nil {
		log.Printf("Error converting threadId to int: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: could not convert threadId to int.")
		return
	}

	// Parse form data (assuming text, maybe attachments)
	text := r.FormValue("text")
	// Escape HTML first to prevent XSS, then process links
	processedText, replyTo, hasPayload := h.TextProcessor.ProcessMessage(domain.Message{Text: text, MessageMetadata: domain.MessageMetadata{Board: shortName, ThreadId: domain.ThreadId(threadId)}})
	if !hasPayload {
		redirectWithError(w, r, errorTargetURL, "Message has empty payload.")
		return
	}
	// TODO: Handle attachments if necessary

	// Prepare backend request data using shared API DTOs
	var domainReplies domain.Replies
	for _, r := range replyTo {
		if r != nil {
			domainReplies = append(domainReplies, &r.Reply)
		}
	}
	backendData := api.CreateMessageRequest{
		Text:    processedText,
		ReplyTo: &domainReplies,
	}

	backendDataJson, err := json.Marshal(backendData)
	if err != nil {
		log.Printf("Error marshalling new reply data: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: could not prepare data.")
		return
	}

	// Create request with authentication
	apiEndpoint := fmt.Sprintf("http://api:8080/v1/%s/%s", shortName, threadIdStr)
	req, err := requestWithCookie(r, "POST", apiEndpoint, bytes.NewBuffer(backendDataJson), "accessToken")
	if err != nil {
		log.Printf("Error creating request for reply creation: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: could not create request.")
		return
	}

	// Send request to backend
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error contacting backend API for reply creation: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: backend unavailable.")
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Error posting reply (status %d)", resp.StatusCode) // Default
		if readErr == nil && len(bodyBytes) > 0 {
			errMsg = fmt.Sprintf("Backend error: %s", string(bodyBytes)) // Use backend message
		} else if readErr != nil {
			log.Printf("Error reading backend error response body: %v", readErr)
		}
		redirectWithError(w, r, targetURL, errMsg)
		return
	}

	// Success: Redirect back to the thread page
	// The reply will appear on the next GET request rendering.
	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}

func ThreadDeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boardShortName := vars["board"]
	threadId := vars["thread"]
	targetURL := fmt.Sprintf("/%s", boardShortName) // Redirect target

	req, err := requestWithCookie(r, "DELETE", fmt.Sprintf("http://api:8080/v1/admin/%s/%s", boardShortName, threadId), nil, "accessToken")
	if err != nil {
		// Use helper for consistency
		redirectWithError(w, r, targetURL, fmt.Sprintf("Internal error: %s", err.Error()))
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error contacting backend API for thread deletion: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: backend unavailable.")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Error deleting thread (status %d)", resp.StatusCode)
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
