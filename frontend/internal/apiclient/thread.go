package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	internal_errors "github.com/itchan-dev/itchan/shared/errors"
	"github.com/itchan-dev/itchan/shared/utils"
)

// === Thread & Message Methods ===

func (c *APIClient) GetThread(r *http.Request, shortName, threadID string) (domain.Thread, error) {
	var thread domain.Thread
	path := fmt.Sprintf("/v1/%s/%s", shortName, threadID)
	resp, err := c.do("GET", path, nil, r.Cookies()...)
	if err != nil {
		return thread, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return thread, &internal_errors.ErrorWithStatusCode{
			Message: fmt.Sprintf("thread /%s/%s not found or access denied", shortName, threadID), StatusCode: resp.StatusCode,
		}
	}

	if err := utils.Decode(resp.Body, &thread); err != nil {
		return thread, fmt.Errorf("cannot decode thread response: %w", err)
	}
	return thread, nil
}

func (c *APIClient) CreateThread(r *http.Request, shortName string, data api.CreateThreadRequest) (string, error) {
	path := fmt.Sprintf("/v1/%s", shortName)
	jsonBody, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal thread data: %w", err)
	}

	resp, err := c.do("POST", path, bytes.NewBuffer(jsonBody), r.Cookies()...)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to create thread: %s", string(bodyBytes))
	}
	return string(bodyBytes), nil // Return new thread ID
}

func (c *APIClient) CreateReply(r *http.Request, shortName, threadID string, data api.CreateMessageRequest) error {
	path := fmt.Sprintf("/v1/%s/%s", shortName, threadID)
	jsonBody, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal reply data: %w", err)
	}

	resp, err := c.do("POST", path, bytes.NewBuffer(jsonBody), r.Cookies()...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create reply: %s", string(bodyBytes))
	}
	return nil
}

func (c *APIClient) DeleteThread(r *http.Request, shortName, threadID string) error {
	path := fmt.Sprintf("/v1/admin/%s/%s", shortName, threadID)
	resp, err := c.do("DELETE", path, nil, r.Cookies()...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete thread: %s", string(bodyBytes))
	}
	return nil
}
