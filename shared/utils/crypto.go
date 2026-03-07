package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

func HashSHA256(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}

// GenerateRandomString generates a cryptographically secure random string
// using the provided charset and length
func GenerateRandomString(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			panic(fmt.Sprintf("failed to generate random string: %v", err))
		}
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

func GenerateConfirmationCode(length int) string {
	// Charset excludes ambiguous characters: 0, O, I, 1
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	return GenerateRandomString(length, charset)
}

// GenerateRandomEmail creates a random @invited.ru email address
// Format: {random_12_chars}@invited.ru
// This is used for invite-based registration to prevent email spoofing
// and ensure users don't get access to private boards
func GenerateRandomEmail() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const length = 12
	return GenerateRandomString(length, charset) + "@invited.ru"
}
