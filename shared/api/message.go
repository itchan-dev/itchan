package api

import (
	"github.com/itchan-dev/itchan/shared/domain"
)

// Request DTOs

type CreateMessageRequest struct {
	Text        string              `json:"text,omitempty"`
	Attachments *domain.Attachments `json:"attachments,omitempty"`
	ReplyTo     *domain.Replies     `json:"reply_to,omitempty"`
}

// Response DTOs

// MessageResponse wraps a full message
type MessageResponse struct {
	domain.Message
	// Add extra API-specific fields here if needed in the future
}

// CreateMessageResponse returns the ID of the created message
type CreateMessageResponse struct {
	Id int64 `json:"id"`
}
