package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/domain"
)

func (c *APIClient) GetMessage(r *http.Request, board, threadID, messageID string) (*http.Response, error) {
	path := fmt.Sprintf("/v1/%s/%s/%s", board, threadID, messageID)
	resp, err := c.do("GET", path, nil, r.Cookies()...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *APIClient) GetMessageParsed(r *http.Request, board, threadID, messageID string) (*domain.Message, error) {
	resp, err := c.GetMessage(r, board, threadID, messageID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get message: %s", string(bodyBytes))
	}

	var message domain.Message
	if err := json.NewDecoder(resp.Body).Decode(&message); err != nil {
		return nil, fmt.Errorf("failed to parse message JSON: %w", err)
	}

	return &message, nil
}

func (c *APIClient) DeleteMessage(r *http.Request, shortName, threadID, messageID string) error {
	path := fmt.Sprintf("/v1/admin/%s/%s/%s", shortName, threadID, messageID)
	resp, err := c.do("DELETE", path, nil, r.Cookies()...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete message: %s", string(bodyBytes))
	}
	return nil
}
