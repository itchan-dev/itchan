package validation

import "errors"

// ErrPayloadTooLarge is returned when the request body exceeds size limits
var ErrPayloadTooLarge = errors.New("payload too large")

// ErrInvalidMimeType is returned when an uploaded file has a disallowed MIME type
var ErrInvalidMimeType = errors.New("invalid MIME type")

// ErrTooManyAttachments is returned when too many files are uploaded
var ErrTooManyAttachments = errors.New("too many attachments")
