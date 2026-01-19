package handler

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	frontend_mw "github.com/itchan-dev/itchan/frontend/internal/middleware"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/validation"
)

// Flash message constants
const (
	flashCookieError   = "flash_error"
	flashCookieSuccess = "flash_success"
)

// setFlash sets a flash message cookie that will be displayed once and then deleted.
// The message is stored as an HTTP-only cookie with a 5-minute expiration.
// Uses base64 encoding to safely store HTML and special characters.
func (h *Handler) setFlash(w http.ResponseWriter, flashType, message string) {
	// Base64 encode the message to safely store HTML and special characters in cookies
	encodedMessage := base64.StdEncoding.EncodeToString([]byte(message))

	cookie := &http.Cookie{
		Name:     flashType,
		Value:    encodedMessage,
		Path:     "/",
		MaxAge:   300, // 5 minutes (enough time for redirect)
		HttpOnly: true,
		Secure:   h.Public.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

// getFlash reads a flash message cookie and immediately deletes it.
// Returns empty string if no flash cookie exists.
func (h *Handler) getFlash(w http.ResponseWriter, r *http.Request, flashType string) template.HTML {
	cookie, err := r.Cookie(flashType)
	if err != nil {
		return ""
	}

	// Delete the cookie immediately
	deleteCookie := &http.Cookie{
		Name:     flashType,
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // Delete immediately
		HttpOnly: true,
		Secure:   h.Public.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, deleteCookie)

	// Base64 decode the message
	decodedBytes, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return ""
	}

	return template.HTML(decodedBytes)
}

// getFlashes reads both error and success flash messages and deletes them.
// This is a convenience function for handlers that need both types.
func (h *Handler) getFlashes(w http.ResponseWriter, r *http.Request) (errMsg template.HTML, successMsg template.HTML) {
	errMsg = h.getFlash(w, r, flashCookieError)
	successMsg = h.getFlash(w, r, flashCookieSuccess)
	return
}

// redirectWithFlash redirects to a URL with a flash message.
// The flash message will be displayed once on the target page and then deleted.
func (h *Handler) redirectWithFlash(w http.ResponseWriter, r *http.Request, targetURL, flashType, message string) {
	h.setFlash(w, flashType, message)
	http.Redirect(w, r, targetURL, http.StatusSeeOther)
}

// redirectWithParams redirects to a target URL after adding the given query parameters.
// It safely handles existing query params and URL fragments (anchors).
// Use this for non-message query params (e.g., email pre-filling).
// For error/success messages, use setFlash() + redirectWithParams() or redirectWithFlash().
func redirectWithParams(w http.ResponseWriter, r *http.Request, targetURL string, params map[string]string) {
	u, err := url.Parse(targetURL)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	query := u.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	u.RawQuery = query.Encode()

	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

// splitAndTrim splits a comma-separated string into a slice of trimmed strings.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseEmail(r *http.Request) string {
	// set default email value from querystring or form value
	var email string
	if r.URL.Query().Get("email") != "" {
		email = r.URL.Query().Get("email")
	} else {
		email = r.FormValue("email")
	}
	return email
}

// processMessageText processes user-submitted text and builds reply references.
// Returns the processed text, domain replies, and whether the message has valid payload.
func (h *Handler) processMessageText(text string, msgMetadata domain.MessageMetadata) (processedText string, replies *domain.Replies, hasPayload bool) {
	processedText, replyTo, hasPayload := h.TextProcessor.ProcessMessage(domain.Message{
		Text:            text,
		MessageMetadata: msgMetadata,
	})

	return processedText, &replyTo, hasPayload
}

// parseAndValidateMultipartForm validates request size and parses the multipart form.
// Returns true if successful, false if validation failed (and redirects with error message).
// This centralizes the duplicate code from thread and board POST handlers.
func (h *Handler) parseAndValidateMultipartForm(w http.ResponseWriter, r *http.Request, errorRedirectURL string) bool {
	// Validate request size and parse multipart form using shared validation
	maxRequestSize := validation.CalculateMaxRequestSize(h.Public.MaxTotalAttachmentSize, 1<<20)
	if err := validation.ValidateAndParseMultipart(r, w, maxRequestSize); err != nil {
		maxSizeMB := validation.FormatSizeMB(h.Public.MaxTotalAttachmentSize)
		errorMsg := fmt.Sprintf("Total attachment size exceeds the limit of %.0f MB. Please reduce the number or size of files.", maxSizeMB)
		h.redirectWithFlash(w, r, errorRedirectURL, flashCookieError, errorMsg)
		return false
	}

	return true
}

// CommonTemplateData holds fields that are common to all page templates.
// Embed this struct in page-specific template data to ensure consistency.
type CommonTemplateData struct {
	Error      template.HTML
	Success    template.HTML
	User       *domain.User
	Validation ValidationData
	CSRFToken  string // CSRF token for form submissions
}

// ValidationData holds all validation constants needed by templates.
// This provides a single source of truth for validation fields across all handlers.
type ValidationData struct {
	// Auth-related validation
	PasswordMinLen      int
	ConfirmationCodeLen int

	// Board-related validation
	BoardNameMaxLen      int
	BoardShortNameMaxLen int

	// Thread-related validation
	ThreadTitleMaxLen int

	// Message-related validation
	MessageTextMaxLen int

	// Attachment-related validation
	MaxAttachmentsPerMessage int
	MaxTotalAttachmentSize   int64
	MaxAttachmentSizeBytes   int64
	AllowedImageMimeTypes    []string
	AllowedVideoMimeTypes    []string

	// User activity page settings
	UserMessagesPageLimit int
}

// NewValidationData creates a ValidationData struct populated from the public config.
// This eliminates the need to manually assign 10+ validation fields in each handler.
func (h *Handler) NewValidationData() ValidationData {
	return ValidationData{
		PasswordMinLen:           h.Public.PasswordMinLen,
		ConfirmationCodeLen:      h.Public.ConfirmationCodeLen,
		BoardNameMaxLen:          h.Public.BoardNameMaxLen,
		BoardShortNameMaxLen:     h.Public.BoardShortNameMaxLen,
		ThreadTitleMaxLen:        h.Public.ThreadTitleMaxLen,
		MessageTextMaxLen:        h.Public.MessageTextMaxLen,
		MaxAttachmentsPerMessage: h.Public.MaxAttachmentsPerMessage,
		MaxTotalAttachmentSize:   h.Public.MaxTotalAttachmentSize,
		MaxAttachmentSizeBytes:   h.Public.MaxAttachmentSizeBytes,
		AllowedImageMimeTypes:    h.Public.AllowedImageMimeTypes,
		AllowedVideoMimeTypes:    h.Public.AllowedVideoMimeTypes,
		UserMessagesPageLimit:    h.Public.UserMessagesPageLimit,
	}
}

// InitCommonTemplateData initializes common template data fields from the request.
// Call this in GET handlers to populate Error, Success, User, and Validation fields.
// Flash messages are automatically read and deleted from cookies.
func (h *Handler) InitCommonTemplateData(w http.ResponseWriter, r *http.Request) CommonTemplateData {
	common := CommonTemplateData{
		User:       mw.GetUserFromContext(r),
		Validation: h.NewValidationData(),
		CSRFToken:  frontend_mw.GetCSRFTokenFromContext(r),
	}
	// Automatically populate flash messages (and delete them)
	common.Error, common.Success = h.getFlashes(w, r)
	return common
}
