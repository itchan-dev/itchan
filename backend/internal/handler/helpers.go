package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/utils"
	"github.com/itchan-dev/itchan/shared/validation"
)

// parseMultipartRequest parses a multipart form request and extracts the JSON payload
// and any uploaded files. Returns the parsed body, pending files, and a cleanup function.
func parseMultipartRequest[T any](w http.ResponseWriter, r *http.Request, h *Handler) (body T, pendingFiles []*domain.PendingFile, cleanup func(), err error) {
	// Validate request size and parse multipart form
	maxRequestSize := validation.CalculateMaxRequestSize(h.cfg.Public.MaxTotalAttachmentSize, 1<<20)
	if err = validation.ValidateAndParseMultipart(r, w, maxRequestSize); err != nil {
		maxSizeMB := validation.FormatSizeMB(h.cfg.Public.MaxTotalAttachmentSize)
		err = fmt.Errorf("%w: total attachment size exceeds the limit of %.0f MB. Please reduce the number or size of files", validation.ErrPayloadTooLarge, maxSizeMB)
		return
	}

	// Get JSON payload from the "json" form field
	jsonPayload := r.FormValue("json")
	if jsonPayload == "" {
		err = fmt.Errorf("missing JSON payload in multipart form")
		return
	}

	if err = utils.DecodeValidate(io.NopCloser(strings.NewReader(jsonPayload)), &body); err != nil {
		return
	}

	// Process uploaded files using shared validation
	files := r.MultipartForm.File["attachments"]
	if len(files) > 0 {
		pendingFiles, err = validation.ValidateAttachments(
			files,
			h.cfg.Public.AllowedImageMimeTypes,
			h.cfg.Public.AllowedVideoMimeTypes,
		)
		if err != nil {
			return
		}

		// Create cleanup function to close all uploaded files
		cleanup = func() {
			for _, pf := range pendingFiles {
				if closer, ok := pf.Data.(io.Closer); ok {
					closer.Close()
				}
			}
		}
	} else {
		cleanup = func() {} // No-op if no files
	}

	return
}

// parseIntParam parses an integer parameter from a string and returns a meaningful error
func parseIntParam(param string, paramName string) (int, error) {
	val, err := strconv.Atoi(param)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: must be an integer", paramName)
	}
	return val, nil
}
