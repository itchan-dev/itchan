package csrf

import (
	"testing"
)

func TestGenerateToken(t *testing.T) {
	token1, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	token2, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Tokens should be different
	if token1 == token2 {
		t.Error("Expected different tokens, got same")
	}

	// Token should have reasonable length
	if len(token1) < 32 {
		t.Errorf("Token too short: %d", len(token1))
	}
}

func TestValidateToken(t *testing.T) {
	token := "test-token-123"

	tests := []struct {
		name        string
		cookieToken string
		formToken   string
		want        bool
	}{
		{"matching tokens", token, token, true},
		{"different tokens", token, "different", false},
		{"empty cookie", "", token, false},
		{"empty form", token, "", false},
		{"both empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateToken(tt.cookieToken, tt.formToken)
			if got != tt.want {
				t.Errorf("ValidateToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
