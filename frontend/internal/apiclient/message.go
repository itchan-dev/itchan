package apiclient

import (
	"fmt"
	"io"
	"net/http"
)

func (c *APIClient) GetMessage(r *http.Request, board, threadID, messageID string) (*http.Response, error) {
	path := fmt.Sprintf("/v1/%s/%s/%s", board, threadID, messageID)
	resp, err := c.do("GET", path, nil, r.Cookies()...)
	if err != nil {
		return nil, err
	}
	return resp, nil
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
