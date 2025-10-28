package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Register sends a registration request. It returns the raw response so the
// handler can check for different success status codes (e.g., 200 vs 202).
func (c *APIClient) Register(email, password string) (*http.Response, error) {
	creds := map[string]string{"email": email, "password": password}
	jsonBody, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal register data: %w", err)
	}

	return c.do("POST", "/v1/auth/register", bytes.NewBuffer(jsonBody))
}

// ConfirmEmail sends an email confirmation code to the backend.
func (c *APIClient) ConfirmEmail(email, code string) error {
	data := map[string]string{"email": email, "confirmation_code": code}
	jsonBody, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal confirmation data: %w", err)
	}

	resp, err := c.do("POST", "/v1/auth/check_confirmation_code", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("confirmation failed: %s", string(bodyBytes))
	}
	return nil
}

// Login sends login credentials. It returns the raw response because the
// handler needs to extract cookies from it.
func (c *APIClient) Login(email, password string) (*http.Response, error) {
	creds := map[string]string{"email": email, "password": password}
	jsonBody, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal login data: %w", err)
	}

	return c.do("POST", "/v1/auth/login", bytes.NewBuffer(jsonBody))
}
