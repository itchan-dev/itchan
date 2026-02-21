package frontend_domain

import "github.com/itchan-dev/itchan/shared/domain"

// CommonTemplateData holds fields that are common to all page templates.
// Available in templates as .Common via the TemplateData wrapper.
type CommonTemplateData struct {
	Error            string
	Success          string
	User             *domain.User
	Validation       ValidationData
	CSRFToken        string // CSRF token for form submissions
	EmailPlaceholder string // Pre-filled email for auth forms (from cookie, not URL)
	DisableMedia     bool   // Hide media (images/videos) and show text placeholders
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

	// Registration restrictions
	AllowedRegistrationDomains []string

	// Thumbnail display sizes
	ThumbnailDisplayOp    int
	ThumbnailDisplayReply int
}
