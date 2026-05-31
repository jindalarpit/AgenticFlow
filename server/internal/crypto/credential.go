package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// CredentialEncryptor handles AES-256-GCM encryption/decryption of provider credentials.
type CredentialEncryptor struct {
	key []byte // 32-byte AES-256 key
}

// NewCredentialEncryptor creates an encryptor from a hex-encoded 32-byte key.
// The hexKey must be exactly 64 hex characters (representing 32 bytes).
func NewCredentialEncryptor(hexKey string) (*CredentialEncryptor, error) {
	if len(hexKey) != 64 {
		return nil, fmt.Errorf("encryption key must be exactly 64 hex characters, got %d", len(hexKey))
	}

	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("encryption key must be valid hex: %w", err)
	}

	if len(key) != 32 {
		return nil, errors.New("decoded encryption key must be 32 bytes")
	}

	return &CredentialEncryptor{key: key}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM with a random 12-byte nonce.
// Returns base64-encoded nonce+ciphertext.
func (e *CredentialEncryptor) Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal appends the ciphertext+tag to nonce, so result is nonce+ciphertext+tag
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded nonce+ciphertext back to plaintext.
func (e *CredentialEncryptor) Decrypt(ciphertext string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, encrypted := data[:nonceSize], data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// MaskCredentialValue masks a credential value for safe display.
// For strings with length >= 4: replaces all characters except the last 4 with asterisks.
// For strings with length < 4: replaces all characters with asterisks.
// For empty strings: returns empty string.
func MaskCredentialValue(value string) string {
	n := len(value)
	if n == 0 {
		return ""
	}
	if n < 4 {
		return strings.Repeat("*", n)
	}
	return strings.Repeat("*", n-4) + value[n-4:]
}
