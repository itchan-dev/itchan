package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
)

// GetMyInvites fetches invite codes created by the authenticated user for the given page
func (c *APIClient) GetMyInvites(r *http.Request, page int) (api.InviteListResponse, error) {
	path := withPage("/v1/invites", page)
	resp, err := c.do("GET", path, nil, r.Cookies()...)
	if err != nil {
		return api.InviteListResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return api.InviteListResponse{}, fmt.Errorf("failed to get invites: %s", string(bodyBytes))
	}

	var result api.InviteListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return api.InviteListResponse{}, fmt.Errorf("failed to parse invites: %w", err)
	}

	if result.Invites == nil {
		result.Invites = []domain.InviteCode{}
	}

	return result, nil
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

	var response api.GenerateInviteResponse

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse invite response: %w", err)
	}

	return &domain.InviteCodeWithPlaintext{
		PlainCode:  response.InviteCode,
		InviteCode: domain.InviteCode{ExpiresAt: response.ExpiresAt},
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
