package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/itchan-dev/itchan/shared/api"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/utils"
)

// RecordReferralAction records a referral action (visit, registration, etc.).
func (c *APIClient) RecordReferralAction(source, action string) error {
	data := api.RecordReferralActionRequest{Source: source, Action: action}
	jsonBody, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal referral action data: %w", err)
	}

	resp, err := c.do("POST", "/v1/auth/referral/action", bytes.NewBuffer(jsonBody), "", "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// GetReferralStats returns referral stats from the admin API.
func (c *APIClient) GetReferralStats(r *http.Request) ([]domain.ReferralActionStats, error) {
	resp, err := c.do("GET", "/v1/admin/referral/stats", nil, getToken(r), getIP(r))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get referral stats: %s", string(bodyBytes))
	}

	var result []domain.ReferralActionStats
	if err := utils.Decode(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode referral stats response: %w", err)
	}

	return result, nil
}
