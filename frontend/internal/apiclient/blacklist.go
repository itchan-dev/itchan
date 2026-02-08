package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
)

// GetBlacklistedUsers returns all blacklisted users
func (c *APIClient) GetBlacklistedUsers(r *http.Request) ([]domain.BlacklistEntry, error) {
	resp, err := c.do("GET", "/v1/admin/blacklist", nil, r.Cookies()...)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get blacklisted users: %s", string(bodyBytes))
	}

	var result api.BlacklistResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode blacklist response: %w", err)
	}

	return result.Users, nil
}

// UnblacklistUser removes a user from the blacklist
func (c *APIClient) UnblacklistUser(r *http.Request, userID string) error {
	path := fmt.Sprintf("/v1/admin/users/%s/blacklist", userID)
	resp, err := c.do("DELETE", path, nil, r.Cookies()...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to unblacklist user: %s", string(bodyBytes))
	}

	return nil
}

// BlacklistUser blacklists a user by their user ID
func (c *APIClient) BlacklistUser(r *http.Request, userID string, reason string) error {
	reqBody := api.BlacklistUserRequest{
		Reason: reason,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal blacklist request: %w", err)
	}

	path := fmt.Sprintf("/v1/admin/users/%s/blacklist", userID)
	resp, err := c.do("POST", path, bytes.NewBuffer(jsonBody), r.Cookies()...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to blacklist user: %s", string(bodyBytes))
	}

	return nil
}
