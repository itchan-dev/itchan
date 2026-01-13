package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
)

// GetUserActivity fetches the authenticated user's activity
func (c *APIClient) GetUserActivity(r *http.Request) (*api.UserActivityResponse, error) {
	// Make API call with user's cookies for authentication
	resp, err := c.do("GET", "/v1/users/me/activity", nil, r.Cookies()...)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user activity: %s", string(bodyBytes))
	}

	// Parse response
	var activity api.UserActivityResponse
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(bodyBytes, &activity); err != nil {
		return nil, fmt.Errorf("failed to parse user activity: %w", err)
	}

	// Initialize empty array if nil
	if activity.Messages == nil {
		activity.Messages = []domain.Message{}
	}

	return &activity, nil
}
