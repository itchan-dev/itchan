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
