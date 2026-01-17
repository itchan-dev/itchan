package api

import (
	"github.com/itchan-dev/itchan/shared/domain"
)

// Request DTOs

type CreateThreadRequest struct {
	Title     string               `json:"title" validate:"required"`
	IsPinned  bool                 `json:"is_pinned,omitempty"`
	OpMessage CreateMessageRequest `json:"op_message"`
}

// Response DTOs

// ThreadMetadataResponse wraps thread metadata
type ThreadMetadataResponse struct {
	domain.ThreadMetadata
	// Add extra API-specific fields here if needed in the future
}

// ThreadResponse wraps a full thread with messages
type ThreadResponse struct {
	domain.Thread
	// Add extra API-specific fields here if needed in the future
}
