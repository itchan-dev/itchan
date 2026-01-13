package api

import "github.com/itchan-dev/itchan/shared/domain"

// Request DTOs shared by backend and frontend handlers

type CreateMessageRequest struct {
	Text        string              `json:"text,omitempty"`
	Attachments *domain.Attachments `json:"attachments,omitempty"`
	ReplyTo     *domain.Replies     `json:"reply_to,omitempty"`
}

type CreateThreadRequest struct {
	Title     string               `json:"title" validate:"required"`
	IsSticky  bool                 `json:"is_sticky,omitempty"`
	OpMessage CreateMessageRequest `json:"op_message"`
}

type CreateBoardRequest struct {
	Name          string         `json:"name" validate:"required"`
	ShortName     string         `json:"short_name" validate:"required"`
	AllowedEmails *domain.Emails `json:"allowed_emails,omitempty"`
}

type BlacklistUserRequest struct {
	Reason string `json:"reason"`
}

type BlacklistResponse struct {
	Users []domain.BlacklistEntry `json:"users"`
}

// UserActivityResponse contains user's recent messages
type UserActivityResponse struct {
	Messages []domain.Message `json:"messages"` // FULLY enriched (Author, Attachments, Replies)
}
