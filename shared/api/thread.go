package api

// Request DTOs

type CreateThreadRequest struct {
	Title     string               `json:"title" validate:"required"`
	IsPinned  bool                 `json:"is_pinned,omitempty"`
	OpMessage CreateMessageRequest `json:"op_message"`
}
