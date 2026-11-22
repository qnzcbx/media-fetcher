package vault

import (
	"bytes"
	"errors"
	"runtime"
	"testing"
)

func TestMockVault_UserIsolation(t *testing.T) {
	vaultAlice := NewMockVault("alice")
	vaultBob := NewMockVault("bob")

	secret := []byte("alice_super_secret_cookie")
	encrypted, err := vaultAlice.Encrypt(secret)
	if err != nil {
		t.Fatalf("Alice encrypt failed: %v", err)
	}

	// Alice decrypts own data
	decrypted, err := vaultAlice.Decrypt(encrypted)
	if err != nil || !bytes.Equal(decrypted, secret) {
		t.Fatalf("Alice decrypt failed: %v, got %s", err, string(decrypted))
	}

	// Bob attempts to decrypt Alice's data -> must fail
	_, err = vaultBob.Decrypt(encrypted)
	if err == nil {
		t.Fatal("Bob should NOT be able to decrypt Alice's data")
	}
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("expected ErrDecryptionFailed, got: %v", err)
	}
}

func TestMockVault_EmptyAndCorrupt(t *testing.T) {
	v := NewMockVault("tester")

	// Empty data
	if _, err := v.Encrypt(nil); !errors.Is(err, ErrEmptyData) {
		t.Fatalf("expected ErrEmptyData for nil, got %v", err)
	}
	if _, err := v.Encrypt([]byte{}); !errors.Is(err, ErrEmptyData) {
		t.Fatalf("expected ErrEmptyData for empty slice, got %v", err)
	}
	if _, err := v.Decrypt([]byte{}); !errors.Is(err, ErrEmptyData) {
		t.Fatalf("expected ErrEmptyData on decrypt empty, got %v", err)
	}

	// Corrupt data
	if _, err := v.Decrypt([]byte("garbage_data_too_short")); err == nil {
		t.Fatal("expected decryption error on corrupt data")
	}
}

func TestWindowsDPAPI_Roundtrip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping Windows DPAPI test on non-windows")
	}

	v := NewDefaultVault()
	secret := []byte("sessdata=secret_token_12345; Path=/; Domain=.bilibili.com")

	ciphertext, err := v.Encrypt(secret)
	if err != nil {
		t.Fatalf("DPAPI Encrypt failed: %v", err)
	}
	if bytes.Equal(ciphertext, secret) {
		t.Fatal("Ciphertext should not equal plaintext")
	}

	plaintext, err := v.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("DPAPI Decrypt failed: %v", err)
	}
	if !bytes.Equal(plaintext, secret) {
		t.Fatalf("Decrypted does not match plaintext: got %s, want %s", string(plaintext), string(secret))
	}

	// Test with optional entropy
	entropy := []byte("media_fetcher_app_salt")
	vEntropy := NewDPAPIVault(entropy)
	cipherEntropy, err := vEntropy.Encrypt(secret)
	if err != nil {
		t.Fatalf("DPAPI with entropy Encrypt failed: %v", err)
	}
	plainEntropy, err := vEntropy.Decrypt(cipherEntropy)
	if err != nil || !bytes.Equal(plainEntropy, secret) {
		t.Fatalf("DPAPI with entropy Decrypt failed: %v", err)
	}

	// Mismatched entropy should fail
	vWrongEntropy := NewDPAPIVault([]byte("wrong_salt"))
	_, err = vWrongEntropy.Decrypt(cipherEntropy)
	if err == nil {
		t.Fatal("DPAPI with wrong entropy should fail decryption")
	}
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("expected ErrDecryptionFailed, got: %v", err)
	}

	// Empty data checks
	if _, err := v.Encrypt([]byte{}); !errors.Is(err, ErrEmptyData) {
		t.Fatalf("expected ErrEmptyData, got %v", err)
	}
	if _, err := v.Decrypt([]byte{}); !errors.Is(err, ErrEmptyData) {
		t.Fatalf("expected ErrEmptyData, got %v", err)
	}

	// Corrupted ciphertext check (tampering with header/version or payload)
	corrupt := append([]byte(nil), ciphertext...)
	if len(corrupt) > 0 {
		corrupt[0] ^= 0xFF
	}
	_, err = v.Decrypt(corrupt)
	if err == nil {
		t.Fatal("corrupt ciphertext should fail decryption")
	}
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("expected ErrDecryptionFailed on corrupt ciphertext, got %v", err)
	}
}
