package api

import "github.com/itchan-dev/itchan/shared/domain"

// Response DTOs

// UserActivityResponse contains user's recent messages
// Messages are FULLY enriched (Author, Attachments, Replies)
type UserActivityResponse struct {
	Messages []domain.Message `json:"messages"`
}
