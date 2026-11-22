// Package vault provides cryptographic storage capabilities for sensitive data.
package vault

import (
	"errors"
	"fmt"
)

var (
	// ErrEmptyData is returned when trying to encrypt or decrypt empty bytes.
	ErrEmptyData = errors.New("vault: data cannot be empty")
	// ErrDecryptionFailed is returned when decryption fails (e.g. invalid key, wrong user, corrupt payload).
	ErrDecryptionFailed = errors.New("vault: decryption failed")
	// ErrCorruptCiphertext is returned when ciphertext payload is malformed or corrupted.
	ErrCorruptCiphertext = errors.New("vault: ciphertext payload is corrupted")
)

// Vault defines the contract for encrypting and decrypting sensitive credentials.
type Vault interface {
	// Encrypt encrypts the given plaintext bytes.
	Encrypt(plaintext []byte) ([]byte, error)
	// Decrypt decrypts the given ciphertext bytes.
	Decrypt(ciphertext []byte) ([]byte, error)
}

// MockVault is an in-memory vault useful for cross-platform unit testing,
// simulating corruption, and simulating cross-user decryption failure.
type MockVault struct {
	UserID    string
	FailOnDec bool
}

// NewMockVault returns a mock vault associated with a specific UserID.
func NewMockVault(userID string) *MockVault {
	return &MockVault{UserID: userID}
}

// Encrypt prepends user signature to simulate user-scoped encryption.
func (m *MockVault) Encrypt(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, ErrEmptyData
	}
	prefix := []byte(fmt.Sprintf("MOCK_VAULT_V1:%s:", m.UserID))
	// Simple obfuscation for mock test
	out := make([]byte, len(prefix)+len(plaintext))
	copy(out, prefix)
	for i, b := range plaintext {
		out[len(prefix)+i] = b ^ 0x5A
	}
	return out, nil
}

// Decrypt verifies user signature and restores plaintext.
func (m *MockVault) Decrypt(ciphertext []byte) ([]byte, error) {
	if m.FailOnDec {
		return nil, ErrDecryptionFailed
	}
	if len(ciphertext) == 0 {
		return nil, ErrEmptyData
	}
	prefix := []byte(fmt.Sprintf("MOCK_VAULT_V1:%s:", m.UserID))
	if len(ciphertext) < len(prefix) || string(ciphertext[:len(prefix)]) != string(prefix) {
		return nil, fmt.Errorf("%w: user mismatch or corrupt header", ErrDecryptionFailed)
	}
	payload := ciphertext[len(prefix):]
	out := make([]byte, len(payload))
	for i, b := range payload {
		out[i] = b ^ 0x5A
	}
	return out, nil
}
