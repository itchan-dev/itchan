package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
)

// Register sends a registration request. It returns the raw response so the
// handler can check for different success status codes (e.g., 200 vs 202).
func (c *APIClient) Register(email, password string) (*http.Response, error) {
	jsonBody, err := json.Marshal(api.RegisterRequest{Email: email, Password: password})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal register data: %w", err)
	}

	return c.do("POST", "/v1/auth/register", bytes.NewBuffer(jsonBody), "")
}

// ConfirmEmail sends an email confirmation code to the backend.
func (c *APIClient) ConfirmEmail(email, code, refSource string) error {
	jsonBody, err := json.Marshal(api.CheckConfirmationCodeRequest{Email: email, ConfirmationCode: code, RefSource: refSource})
	if err != nil {
		return fmt.Errorf("failed to marshal confirmation data: %w", err)
	}

	resp, err := c.do("POST", "/v1/auth/check_confirmation_code", bytes.NewBuffer(jsonBody), "")
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

// Login sends login credentials. It returns the raw response so the handler
// can parse the access token from the JSON body.
func (c *APIClient) Login(email, password string) (*http.Response, error) {
	jsonBody, err := json.Marshal(api.LoginRequest{Email: email, Password: password})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal login data: %w", err)
	}

	return c.do("POST", "/v1/auth/login", bytes.NewBuffer(jsonBody), "")
}

// RegisterWithInvite sends an invite code registration request to the backend.
// It returns the generated random email address on success.
func (c *APIClient) RegisterWithInvite(inviteCode, password, refSource string) (string, error) {
	jsonBody, err := json.Marshal(api.RegisterWithInviteRequest{InviteCode: inviteCode, Password: password, RefSource: refSource})
	if err != nil {
		return "", fmt.Errorf("failed to marshal invite registration data: %w", err)
	}

	resp, err := c.do("POST", "/v1/auth/register_with_invite", bytes.NewBuffer(jsonBody), "")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registration failed: %s", string(bodyBytes))
	}

	var response api.RegisterWithInviteResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return "", fmt.Errorf("failed to parse registration response: %w", err)
	}

	return response.Email, nil
}
