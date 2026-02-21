package handler

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	frontend_mw "github.com/itchan-dev/itchan/frontend/internal/middleware"
	"github.com/itchan-dev/itchan/shared/domain"
	mw "github.com/itchan-dev/itchan/shared/middleware"
	"github.com/itchan-dev/itchan/shared/validation"
)

// Flash message constants
const (
	flashCookieError   = "flash_error"
	flashCookieSuccess = "flash_success"
	emailPrefillCookie = "email_prefill"
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
func (h *Handler) getFlash(w http.ResponseWriter, r *http.Request, flashType string) string {
	cookie, err := r.Cookie(flashType)
	if err != nil {
		return ""
	}

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

	return string(decodedBytes)
}

// getFlashes reads both error and success flash messages and deletes them.
// This is a convenience function for handlers that need both types.
func (h *Handler) getFlashes(w http.ResponseWriter, r *http.Request) (errMsg string, successMsg string) {
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

// processMessageText processes user-submitted text and builds reply references.
// Returns the processed text, domain replies, and whether the message has valid payload.
func (h *Handler) processMessageText(text string, msgMetadata domain.MessageMetadata) (processedText string, replies *domain.Replies, hasPayload bool, err error) {
	processedText, replyTo, hasPayload, err := h.TextProcessor.ProcessMessage(domain.Message{
		Text:            text,
		MessageMetadata: msgMetadata,
	})

	return processedText, &replyTo, hasPayload, err
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

// NewValidationData creates a ValidationData struct populated from the public config.
func (h *Handler) newValidationData() frontend_domain.ValidationData {
	return frontend_domain.ValidationData{
		PasswordMinLen:             h.Public.PasswordMinLen,
		ConfirmationCodeLen:        h.Public.ConfirmationCodeLen,
		BoardNameMaxLen:            h.Public.BoardNameMaxLen,
		BoardShortNameMaxLen:       h.Public.BoardShortNameMaxLen,
		ThreadTitleMaxLen:          h.Public.ThreadTitleMaxLen,
		MessageTextMaxLen:          h.Public.MessageTextMaxLen,
		MaxAttachmentsPerMessage:   h.Public.MaxAttachmentsPerMessage,
		MaxTotalAttachmentSize:     h.Public.MaxTotalAttachmentSize,
		MaxAttachmentSizeBytes:     h.Public.MaxAttachmentSizeBytes,
		AllowedImageMimeTypes:      h.Public.AllowedImageMimeTypes,
		AllowedVideoMimeTypes:      h.Public.AllowedVideoMimeTypes,
		UserMessagesPageLimit:      h.Public.UserMessagesPageLimit,
		AllowedRegistrationDomains: h.Public.AllowedRegistrationDomains,
	}
}

// InitCommonTemplateData initializes common template data fields from the request.
// Flash messages and email prefill are automatically read and deleted from cookies.
func (h *Handler) initCommonTemplateData(w http.ResponseWriter, r *http.Request) frontend_domain.CommonTemplateData {
	common := frontend_domain.CommonTemplateData{
		User:       mw.GetUserFromContext(r),
		Validation: h.newValidationData(),
		CSRFToken:  frontend_mw.GetCSRFTokenFromContext(r),
	}
	// Automatically populate flash messages (and delete them)
	common.Error, common.Success = h.getFlashes(w, r)
	// Read email prefill cookie (reuses flash pattern)
	common.EmailPlaceholder = h.getFlash(w, r, emailPrefillCookie)
	// Read disable_media preference cookie
	if c, err := r.Cookie("disable_media"); err == nil && c.Value == "1" {
		common.DisableMedia = true
	}
	return common
}
