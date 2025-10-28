package apiclient

import (
	"fmt"
	"io"
	"net/http"
)

// APIClient struct handles all communication with the backend API.
type APIClient struct {
	BaseURL    string
	HttpClient *http.Client
}

// NewAPIClient creates a new client for interacting with the backend.
func New(baseURL string) *APIClient {
	return &APIClient{
		BaseURL:    baseURL,
		HttpClient: &http.Client{},
	}
}

// do is the single, unified helper for making API requests.
// It accepts an optional slice of cookies to be attached to the request.
func (c *APIClient) do(method, path string, body io.Reader, cookies ...*http.Cookie) (*http.Response, error) {
	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create API request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Add any cookies that were passed in.
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("backend unavailable: %w", err)
	}
	return resp, nil
}
