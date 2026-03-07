package apiclient

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
// Pass the JWT token for authenticated endpoints; empty string for public endpoints.
// Pass the real client IP (from X-Real-IP) to forward rate limiting to the backend; empty string to skip.
func (c *APIClient) do(r *http.Request, method, path string, body io.Reader) (*http.Response, error) {
	token := getToken(r)
	ip := getIP(r)
	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create API request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if ip != "" {
		req.Header.Set("X-Real-IP", ip)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("backend unavailable: %w", err)
	}
	return resp, nil
}

// getToken extracts the JWT token from the incoming browser request cookie.
func getToken(r *http.Request) string {
	if c, err := r.Cookie("access_token"); err == nil {
		return c.Value
	}
	return ""
}

// getIP extracts the real client IP from the incoming browser request.
// nginx sets X-Real-IP to the original client IP before proxying to the frontend.
func getIP(r *http.Request) string {
	return r.Header.Get("X-Real-IP")
}

func withPage(path string, page int) string {
	if page <= 1 {
		return path
	}
	return path + "?" + url.Values{"page": {strconv.Itoa(page)}}.Encode()
}
