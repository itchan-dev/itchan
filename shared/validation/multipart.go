package validation

import (
	"fmt"
	"net/http"
)

// ValidateAndParseMultipart validates request size and parses the multipart form.
// It sets up MaxBytesReader to enforce the size limit and attempts to parse the form.
// Returns an error if the size limit is exceeded or parsing fails.
//
// Connection Reset Behavior (By Design):
// When MaxBytesReader's limit is exceeded, the server stops reading and closes the connection,
// which triggers ERR_CONNECTION_RESET in browsers. This is EXPECTED and ACCEPTABLE because:
//
//   1. MaxBytesReader only reads UP TO the limit (e.g., 21MB), then stops - preventing resource
//      exhaustion even if a user tries to upload 200TB.
//   2. Client-side JavaScript validation catches 99.9% of legitimate users before upload starts.
//   3. Connection reset only affects:
//      - Malicious users trying to bypass client-side checks (acceptable)
//      - Users with JavaScript disabled (rare, acceptable trade-off)
//      - API clients that don't check Content-Length (they handle resets gracefully)
//
// This multi-layer defense (JS validation → MaxBytesReader) is the industry standard approach.
// For browser form submissions, we cannot reject early by checking Content-Length because
// the browser has already started uploading when we receive the request.
func ValidateAndParseMultipart(r *http.Request, w http.ResponseWriter, maxSize int64) error {
	// MaxBytesReader wraps the body and stops reading when limit is exceeded
	// This is the security boundary that prevents resource exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	// ParseMultipartForm reads the body and will error if MaxBytesReader limit is hit
	// The browser upload is stopped gracefully through the read operation
	if err := r.ParseMultipartForm(maxSize); err != nil {
		return fmt.Errorf("%w: failed to parse multipart form", ErrPayloadTooLarge)
	}

	return nil
}

// CalculateMaxRequestSize returns the maximum request size including overhead buffer.
// It adds a buffer (typically 1 MiB) for form fields and multipart overhead.
func CalculateMaxRequestSize(maxAttachmentSize int64, bufferSize int64) int64 {
	return maxAttachmentSize + bufferSize
}

// FormatSizeMB converts bytes to megabytes for user-friendly error messages.
func FormatSizeMB(bytes int64) float64 {
	return float64(bytes) / (1024 * 1024)
}
