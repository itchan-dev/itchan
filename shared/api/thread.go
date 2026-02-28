package api

import "time"

// Request DTOs

type CreateThreadRequest struct {
	Title     string               `json:"title" validate:"required"`
	IsPinned  bool                 `json:"is_pinned,omitempty"`
	OpMessage CreateMessageRequest `json:"op_message"`
}

// Response DTOs

type CreateThreadResponse struct {
	ID int64 `json:"id"`
}

type TogglePinnedThreadResponse struct {
	IsPinned bool `json:"is_pinned"`
}

type LastModifiedResponse struct {
	LastModifiedAt time.Time `json:"last_modified_at"`
}
