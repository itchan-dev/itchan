package apiclient

import (
	"fmt"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/utils"
)

// GetUserActivity fetches the authenticated user's activity
func (c *APIClient) GetUserActivity(r *http.Request) ([]domain.Message, error) {
	resp, err := c.do("GET", "/v1/users/me/activity", nil, r.Cookies()...)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user activity: %s", string(bodyBytes))
	}

	var messages []domain.Message
	if err := utils.Decode(resp.Body, &messages); err != nil {
		return nil, fmt.Errorf("failed to parse user activity: %w", err)
	}

	if messages == nil {
		messages = []domain.Message{}
	}

	return messages, nil
}
