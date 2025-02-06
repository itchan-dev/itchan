package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
)

var (
	hashPepper = generatePepper()
)

func generatePepper() string {
	return uuid.New().String() + "-" + uuid.New().String()
}

func HashSHA256(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))

	mac := hmac.New(sha256.New, []byte(hashPepper))
	mac.Write([]byte(input))

	hashBytes := mac.Sum(nil)

	// Convert to hexadecimal
	return hex.EncodeToString(hashBytes)
}
