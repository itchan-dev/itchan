package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/domain"
)

// GetMyInvites fetches all invite codes created by the authenticated user
func (c *APIClient) GetMyInvites(r *http.Request) ([]domain.InviteCode, error) {
	resp, err := c.do("GET", "/v1/invites", nil, r.Cookies()...)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get invites: %s", string(bodyBytes))
	}

	var invites []domain.InviteCode
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(bodyBytes, &invites); err != nil {
		return nil, fmt.Errorf("failed to parse invites: %w", err)
	}

	if invites == nil {
		invites = []domain.InviteCode{}
	}

	return invites, nil
}

// GenerateInvite creates a new invite code for the authenticated user
func (c *APIClient) GenerateInvite(r *http.Request) (*domain.InviteCodeWithPlaintext, error) {
	// Make API call (no request body needed)
	resp, err := c.do("POST", "/v1/invites", nil, r.Cookies()...)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to generate invite: %s", string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Response format: {"invite_code": "...", "expires_at": "..."}
	var response struct {
		InviteCode string `json:"invite_code"`
		ExpiresAt  string `json:"expires_at"`
	}

	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse invite response: %w", err)
	}

	// For now, return a simplified structure with just the plain code
	// The full InviteCode will be fetched when refreshing the list
	return &domain.InviteCodeWithPlaintext{
		PlainCode: response.InviteCode,
	}, nil
}

// RevokeInvite deletes an unused invite code owned by the authenticated user
func (c *APIClient) RevokeInvite(r *http.Request, codeHash string) error {
	path := fmt.Sprintf("/v1/invites/%s", codeHash)
	resp, err := c.do("DELETE", path, nil, r.Cookies()...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to revoke invite: %s", string(bodyBytes))
	}

	return nil
}
