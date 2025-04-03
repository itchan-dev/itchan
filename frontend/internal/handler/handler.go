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
	"net/url"
	"strings" // Added for splitAndTrim if not already imported elsewhere

	"github.com/gorilla/mux"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/utils"
)

type credentials struct {
	Email    string `validate:"required" json:"email"`
	Password string `validate:"required" json:"password"`
}

type Handler struct {
	Templates map[string]*template.Template
}

func New(templates map[string]*template.Template) *Handler {
	return &Handler{
		Templates: templates,
	}
}

func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/favicon.ico")
}

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
		Boards []domain.Board
		Error  template.HTML
		User   *domain.User
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

func (h *Handler) RegisterGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Error template.HTML
		User  *domain.User // Might be nil if not logged in
	}
	templateData.User = mw.GetUserFromContext(r)
	templateData.Error, _ = parseMessagesFromQuery(r) // Get error from query param
	h.renderTemplate(w, "register.html", templateData)
}

func (h *Handler) RegisterPostHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := "/register" // Redirect target on error
	successURL := "/check_confirmation_code"

	// Parse form data
	email := r.FormValue("email")
	password := r.FormValue("password")
	creds := credentials{Email: email, Password: password}

	// Prepare backend request data
	credsJson, err := json.Marshal(creds)
	if err != nil {
		log.Printf("Error marshalling registration data: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: could not prepare data.")
		return
	}

	// Send request to backend
	resp, err := http.Post("http://api:8080/v1/auth/register", "application/json", bytes.NewBuffer(credsJson))
	if err != nil {
		log.Printf("Error contacting backend API for registration: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: backend unavailable.")
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusTooEarly {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Registration failed (status %d)", resp.StatusCode) // Default message
		if readErr == nil && len(bodyBytes) > 0 {
			errMsg = fmt.Sprintf("Error: %s", string(bodyBytes))
		} else if readErr != nil {
			log.Printf("Error reading backend error response body: %v", readErr)
		}
		redirectWithError(w, r, targetURL, errMsg)
		return
	}

	// Handle specific case: confirmation needed
	if resp.StatusCode == http.StatusTooEarly {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errMsg := "Confirmation required." // Default message
		if readErr == nil && len(bodyBytes) > 0 {
			// Include the backend message and a link hint in the error for the redirect
			errMsg = fmt.Sprintf(`%s Please check your email or use the confirmation page. <a href="/check_confirmation_code?email=%s">Go to Confirmation</a>`, string(bodyBytes), url.QueryEscape(email))
		} else if readErr != nil {
			log.Printf("Error reading backend StatusTooEarly response body: %v", readErr)
		}
		// Redirect back to register page with this specific error message
		redirectWithError(w, r, targetURL, errMsg)
		return
	}

	// Success (StatusOK): Redirect to confirmation page with email pre-filled
	finalSuccessURL := fmt.Sprintf("%s?email=%s", successURL, url.QueryEscape(email))
	http.Redirect(w, r, finalSuccessURL, http.StatusSeeOther)
}

func (h *Handler) ConfirmEmailGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Error            template.HTML
		Success          template.HTML
		EmailPlaceholder string
		User             *domain.User
	}
	templateData.User = mw.GetUserFromContext(r)
	templateData.EmailPlaceholder = parseEmail(r)                        // Get email from query param
	templateData.Error, templateData.Success = parseMessagesFromQuery(r) // Get messages

	// Customize success message if needed based on query param
	if r.URL.Query().Get("success") == "confirmed" {
		templateData.Success = template.HTML(`Success! You can now <a href="/login">login</a>.`)
	}

	h.renderTemplate(w, "check_confirmation_code.html", templateData)
}

func (h *Handler) ConfirmEmailPostHandler(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	email := r.FormValue("email")
	code := r.FormValue("confirmation_code")

	// Redirect target on error (back to the confirmation page with email)
	targetURL := fmt.Sprintf("/check_confirmation_code?email=%s", url.QueryEscape(email))

	// Prepare backend request data
	backendData := struct {
		Email            string `json:"email"`
		ConfirmationCode string `json:"confirmation_code"`
	}{Email: email, ConfirmationCode: code}

	backendDataJson, err := json.Marshal(backendData)
	if err != nil {
		log.Printf("Error marshalling confirmation data: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: could not prepare data.")
		return
	}

	// Send request to backend
	resp, err := http.Post("http://api:8080/v1/auth/check_confirmation_code", "application/json", bytes.NewBuffer(backendDataJson))
	if err != nil {
		log.Printf("Error contacting backend API for confirmation: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: backend unavailable.")
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Confirmation failed (status %d)", resp.StatusCode) // Default message
		if readErr == nil && len(bodyBytes) > 0 {
			errMsg = string(bodyBytes)
		} else if readErr != nil {
			log.Printf("Error reading backend error response body: %v", readErr)
		}
		redirectWithError(w, r, targetURL, errMsg)
		return
	}

	// Success: Redirect back to confirmation page with a success flag/message
	// Using a simple flag like "confirmed"
	redirectWithSuccess(w, r, targetURL, "confirmed")
	// Or redirect to login page directly:
	// http.Redirect(w, r, "/login?success=confirmed", http.StatusSeeOther)
}

func (h *Handler) LoginGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Error            template.HTML
		User             *domain.User
		EmailPlaceholder string
	}
	templateData.User = mw.GetUserFromContext(r)
	templateData.Error, _ = parseMessagesFromQuery(r)          // Get error message
	templateData.EmailPlaceholder = r.URL.Query().Get("email") // Pre-fill email if redirected with it
	if templateData.EmailPlaceholder == "" {
		// Fallback if not passed in query
		templateData.EmailPlaceholder = parseEmail(r)
	}

	h.renderTemplate(w, "login.html", templateData)
}

func (h *Handler) LoginPostHandler(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	email := r.FormValue("email")
	password := r.FormValue("password")

	// Redirect target on error (back to login page, preserving email)
	targetURL := fmt.Sprintf("/login?email=%s", url.QueryEscape(email))
	successURL := "/" // Redirect target on success

	// Prepare backend request data
	backendData := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{Email: email, Password: password}

	backendDataJson, err := json.Marshal(backendData)
	if err != nil {
		log.Printf("Error marshalling login data: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: could not prepare data.")
		return
	}

	// Send request to backend
	resp, err := http.Post("http://api:8080/v1/auth/login", "application/json", bytes.NewBuffer(backendDataJson))
	if err != nil {
		log.Printf("Error contacting backend API for login: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: backend unavailable.")
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("Login failed (status %d)", resp.StatusCode) // Default message
		if readErr == nil && len(bodyBytes) > 0 {
			errMsg = string(bodyBytes)
		} else if readErr != nil {
			log.Printf("Error reading backend error response body: %v", readErr)
		}
		redirectWithError(w, r, targetURL, errMsg)
		return
	}

	// Success: Forward cookies from backend response
	for _, cookie := range resp.Cookies() {
		// Make sure critical cookies like session tokens are HttpOnly
		// The backend should set HttpOnly=true, this just forwards it.
		http.SetCookie(w, cookie)
	}

	// Redirect to the success URL (index page)
	http.Redirect(w, r, successURL, http.StatusSeeOther)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Clear the access token cookie
	cookie := &http.Cookie{
		Path:     "/",
		Name:     "accessToken",
		Value:    "",
		MaxAge:   -1, // Expire immediately
		HttpOnly: true,
		// Secure: true, // Add this if using HTTPS
		// SameSite: http.SameSiteLaxMode, // Or Strict
	}
	http.SetCookie(w, cookie)

	// Redirect to login page
	http.Redirect(w, r, "/login", http.StatusSeeOther) // Use SeeOther after logout action
}

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

func getBoardPreview(r *http.Request, shortName string, page string) (*domain.Board, error) {
	// Construct URL carefully, handling empty page param
	apiURL := fmt.Sprintf("http://api:8080/v1/%s", shortName)
	if page != "" {
		apiURL = fmt.Sprintf("%s?page=%s", apiURL, url.QueryEscape(page))
	}

	req, err := requestWithCookie(r, "GET", apiURL, nil, "accessToken")
	if err != nil {
		return nil, errors.New("internal error: request creation failed")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error getting board preview from API: %v", err)
		return nil, errors.New("internal error: backend unavailable")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("board /%s not found", shortName) // Specific error for 404
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("API backend error (%d) getting board preview: %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("internal error: backend returned status %d", resp.StatusCode)
	}

	var board domain.Board
	if err := utils.Decode(resp.Body, &board); err != nil {
		return nil, errors.New("internal error: cannot decode response")
	}
	return &board, nil
}

func (h *Handler) BoardGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Board *domain.Board
		Error template.HTML
		User  *domain.User
	}
	templateData.User = mw.GetUserFromContext(r)
	shortName := mux.Vars(r)["board"]
	page := r.URL.Query().Get("page")
	templateData.Error, _ = parseMessagesFromQuery(r) // Get error from query param

	board, err := getBoardPreview(r, shortName, page)
	if err != nil {
		// Distinguish between not found and other errors
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	templateData.Board = board

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

	// Prepare backend request data
	backendData := struct {
		Title string `json:"title"`
		Text  string `json:"text"`
		// Attachments *domain.Attachments `json:"attachments"` // Add if handling attachments
	}{Title: title, Text: text}

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

func (h *Handler) ThreadGetHandler(w http.ResponseWriter, r *http.Request) {
	var templateData struct {
		Thread *domain.Thread
		Error  template.HTML
		User   *domain.User
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
	templateData.Thread = &thread

	h.renderTemplate(w, "thread.html", templateData)
}

// POST handler for creating a new reply within a thread
func (h *Handler) ThreadPostHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortName := vars["board"]
	threadId := vars["thread"]

	// Redirect target (back to the same thread page on success or error)
	targetURL := fmt.Sprintf("/%s/%s", shortName, threadId)

	// Parse form data (assuming text, maybe attachments)
	text := r.FormValue("text")
	// TODO: Handle attachments if necessary

	// Prepare backend request data
	backendData := struct {
		Text string `json:"text"`
		// Attachments *domain.Attachments `json:"attachments"` // Add if handling attachments
	}{Text: text}

	backendDataJson, err := json.Marshal(backendData)
	if err != nil {
		log.Printf("Error marshalling new reply data: %v", err)
		redirectWithError(w, r, targetURL, "Internal error: could not prepare data.")
		return
	}

	// Create request with authentication
	apiEndpoint := fmt.Sprintf("http://api:8080/v1/%s/%s", shortName, threadId)
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
