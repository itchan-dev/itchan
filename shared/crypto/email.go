package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrInvalidKey        = errors.New("encryption key must be 32 bytes for AES-256")
	ErrInvalidCiphertext = errors.New("ciphertext is too short or corrupted")
	ErrInvalidEmail      = errors.New("invalid email format")
)

// EmailCrypto handles encryption and decryption of email addresses
type EmailCrypto struct {
	key []byte
}

// NewEmailCrypto creates a new EmailCrypto instance with the provided key
// The key should be 32 bytes for AES-256
func NewEmailCrypto(keyBase64 string) (*EmailCrypto, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encryption key: %w", err)
	}

	if len(key) != 32 {
		return nil, ErrInvalidKey
	}

	return &EmailCrypto{key: key}, nil
}

// Encrypt encrypts an email address using AES-256-GCM
// Returns the encrypted bytes with the nonce prepended
func (e *EmailCrypto) Encrypt(email string) ([]byte, error) {
	// Normalize email to lowercase
	email = strings.ToLower(strings.TrimSpace(email))

	if email == "" {
		return nil, ErrInvalidEmail
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and prepend nonce to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, []byte(email), nil)

	return ciphertext, nil
}

// Decrypt decrypts an encrypted email address
func (e *EmailCrypto) Decrypt(ciphertext []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", ErrInvalidCiphertext
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	// Extract nonce and ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// Hash creates a deterministic SHA-256 hash of an email for lookups
// Always normalizes email to lowercase before hashing
func (e *EmailCrypto) Hash(email string) []byte {
	// Normalize email to lowercase
	email = strings.ToLower(strings.TrimSpace(email))

	hash := sha256.Sum256([]byte(email))
	return hash[:]
}

// ExtractDomain extracts the domain portion from an email address
// Returns empty string if email is invalid
func (e *EmailCrypto) ExtractDomain(email string) (string, error) {
	// Normalize email to lowercase
	email = strings.ToLower(strings.TrimSpace(email))

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "", ErrInvalidEmail
	}

	return parts[1], nil
}

// GenerateKey generates a random 32-byte key for AES-256 and returns it as base64
// This is a utility function for generating new encryption keys
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
