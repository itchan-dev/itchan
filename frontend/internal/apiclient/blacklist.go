package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
)

// BlacklistUser blacklists a user by their user ID
func (c *APIClient) BlacklistUser(r *http.Request, userID string, reason string) error {
	// Create request body
	reqBody := api.BlacklistUserRequest{
		Reason: reason,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal blacklist request: %w", err)
	}

	// Make API call
	path := fmt.Sprintf("/v1/admin/users/%s/blacklist", userID)
	resp, err := c.do("POST", path, bytes.NewBuffer(jsonBody), r.Cookies()...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to blacklist user: %s", string(bodyBytes))
	}

	return nil
}
