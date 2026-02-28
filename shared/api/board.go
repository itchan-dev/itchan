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
