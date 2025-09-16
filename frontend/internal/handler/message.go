package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func MessageDeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	boardShortName := vars["board"]
	threadId := vars["thread"]
	messageId := vars["message"]
	targetURL := fmt.Sprintf("/%s/%s", boardShortName, threadId) // Redirect target

	req, err := requestWithCookie(r, "DELETE", fmt.Sprintf("http://api:8080/v1/admin/%s/%s/%s", boardShortName, threadId, messageId), nil, "accessToken")
	if err != nil {
		// Use helper for consistency
		redirectWithError(w, r, targetURL, fmt.Sprintf("Internal error: %s", err.Error()))
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error contacting backend API for message deletion: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: backend unavailable.")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Error deleting message (status %d)", resp.StatusCode)
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

// MessagePreviewHandler proxies message API requests for JavaScript preview
func MessagePreviewHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	board := vars["board"]
	threadId := vars["thread"]
	messageId := vars["message"]

	// Create request to backend API
	apiURL := fmt.Sprintf("http://api:8080/v1/%s/%s/%s", board, threadId, messageId)
	req, err := requestWithCookie(r, "GET", apiURL, nil, "accessToken")
	if err != nil {
		http.Error(w, "Internal error: could not create request", http.StatusInternalServerError)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error getting message data from API: %v", err)
		http.Error(w, "Internal error: backend unavailable", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Set appropriate headers for JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Forward the response status and body
	w.WriteHeader(resp.StatusCode)

	// Copy response body to client
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error copying response body: %v", err)
	}
}
