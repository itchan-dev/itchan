package api

import (
	"github.com/itchan-dev/itchan/shared/domain"
)

// Request DTOs

type CreateMessageRequest struct {
	Text            string              `json:"text,omitempty"`
	ShowEmailDomain bool                `json:"show_email_domain,omitempty"`
	Attachments     *domain.Attachments `json:"attachments,omitempty"`
	ReplyTo         *domain.Replies     `json:"reply_to,omitempty"`
}

// Response DTOs

// CreateMessageResponse returns the ID of the created message and its page
type CreateMessageResponse struct {
	Id   int64 `json:"id"`
	Page int   `json:"page"` // Page number where the message appears
}
