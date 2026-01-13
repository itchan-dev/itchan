package service

import (
	"fmt"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
)

// UserActivityService provides methods for fetching user activity data
type UserActivityService interface {
	GetUserActivity(userId domain.UserId) (*api.UserActivityResponse, error)
}

// UserActivity implements UserActivityService
type UserActivity struct {
	storage UserActivityStorage
	cfg     *config.Public
}

// UserActivityStorage defines storage interface for user activity operations
type UserActivityStorage interface {
	GetUserMessages(userId domain.UserId, limit int) ([]domain.Message, error)
}

// NewUserActivity creates a new UserActivity service
func NewUserActivity(storage UserActivityStorage, cfg *config.Public) UserActivityService {
	return &UserActivity{
		storage: storage,
		cfg:     cfg,
	}
}

// GetUserActivity fetches user's recent messages
func (s *UserActivity) GetUserActivity(userId domain.UserId) (*api.UserActivityResponse, error) {
	limit := s.cfg.UserMessagesPageLimit

	// Fetch user's messages
	messages, err := s.storage.GetUserMessages(userId, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get user messages: %w", err)
	}

	return &api.UserActivityResponse{
		Messages: messages,
	}, nil
}
