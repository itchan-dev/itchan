package api

import (
	"github.com/itchan-dev/itchan/shared/domain"
)

// Request DTOs

type CreateBoardRequest struct {
	Name          string         `json:"name" validate:"required"`
	ShortName     string         `json:"short_name" validate:"required"`
	AllowedEmails *domain.Emails `json:"allowed_emails,omitempty"`
}

// Response DTOs

// BoardMetadataResponse wraps board metadata
// Embed domain.BoardMetadata to get all fields
type BoardMetadataResponse struct {
	domain.BoardMetadata
	// Add extra API-specific fields here if needed in the future
}

// BoardResponse wraps a full board with threads
type BoardResponse struct {
	domain.Board
	// Add extra API-specific fields here if needed in the future
}

// BoardListResponse wraps a list of boards
type BoardListResponse struct {
	Boards []BoardMetadataResponse `json:"boards"`
}
