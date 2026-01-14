package csrf

import (
	"crypto/rand"
	"encoding/base64"
)

const TokenLength = 32 // bytes

// GenerateToken creates a cryptographically secure random token
func GenerateToken() (string, error) {
	bytes := make([]byte, TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// ValidateToken compares the cookie token with the form token
func ValidateToken(cookieToken, formToken string) bool {
	if cookieToken == "" || formToken == "" {
		return false
	}
	return cookieToken == formToken
}
