package handler

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"

	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/validation"
)

// redirectWithParams correctly redirects to a target URL after adding the given
// query parameters. It safely handles existing query params and URL fragments (anchors).
func redirectWithParams(w http.ResponseWriter, r *http.Request, targetURL string, params map[string]string) {
	// 1. Parse the base URL to separate path, query, and fragment.
	u, err := url.Parse(targetURL)
	if err != nil {
		// If parsing fails, it's a server-side programming error.
		// Log it and fall back to a simple redirect to a safe URL.
		log.Printf("ERROR: Failed to parse redirect URL '%s': %v", targetURL, err)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// 2. Get the existing query values from the URL.
	query := u.Query()

	// 3. Add the new parameters, overwriting any existing keys.
	for key, value := range params {
		query.Set(key, value)
	}

	// 4. Encode the modified query and assign it back to the URL object.
	u.RawQuery = query.Encode()

	// 5. Perform the redirect using the reassembled URL string.
	// The u.String() method correctly combines the path, new query, and original fragment.
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

// parseMessagesFromQuery extracts and decodes error/success messages from URL query parameters.
func parseMessagesFromQuery(r *http.Request) (errMsg template.HTML, successMsg template.HTML) {
	if errorParam, err := url.QueryUnescape(r.URL.Query().Get("error")); err == nil && errorParam != "" {
		errMsg = template.HTML(template.HTMLEscapeString(errorParam))
	}

	if successParam, err := url.QueryUnescape(r.URL.Query().Get("success")); err == nil && successParam != "" {
		successMsg = template.HTML(template.HTMLEscapeString(successParam))
	}
	return
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

	var domainReplies domain.Replies
	for _, rep := range replyTo {
		if rep != nil {
			domainReplies = append(domainReplies, &rep.Reply)
		}
	}

	return processedText, &domainReplies, hasPayload
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
		redirectWithParams(w, r, errorRedirectURL, map[string]string{"error": errorMsg})
		return false
	}

	return true
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
	}
}

// MessageViewContext contains the rendering context for a message.
type MessageViewContext struct {
	ShowDeleteButton bool
	ShowReplyButton  bool
	ExtraClasses     string
	Subject          string
}

// PrepareMessageView creates a MessageView with rendering context.
// URL building is delegated to templates using printf.
func PrepareMessageView(msg *frontend_domain.Message, ctx MessageViewContext) *frontend_domain.MessageView {
	return &frontend_domain.MessageView{
		Message:          msg,
		ExtraClasses:     ctx.ExtraClasses,
		ShowDeleteButton: ctx.ShowDeleteButton,
		ShowReplyButton:  ctx.ShowReplyButton,
		Subject:          ctx.Subject,
	}
}
