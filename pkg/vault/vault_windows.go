//go:build windows

package vault

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DPAPIVault implements Vault using Windows Data Protection API (DPAPI).
// It strictly uses the current user's credentials and refuses machine-wide scope.
type DPAPIVault struct {
	entropy []byte
}

// NewDPAPIVault creates a new Windows DPAPI-based vault.
// Optional entropy can be provided for additional defense-in-depth, or nil for default user scope.
func NewDPAPIVault(optionalEntropy []byte) *DPAPIVault {
	var ent []byte
	if len(optionalEntropy) > 0 {
		ent = make([]byte, len(optionalEntropy))
		copy(ent, optionalEntropy)
	}
	return &DPAPIVault{entropy: ent}
}

// NewDefaultVault returns the platform's standard Vault implementation.
func NewDefaultVault() Vault {
	return NewDPAPIVault(nil)
}

// Encrypt encrypts data using CryptProtectData under the current user's context.
// Flags parameter is 0, explicitly avoiding CRYPTPROTECT_LOCAL_MACHINE (0x4).
func (v *DPAPIVault) Encrypt(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, ErrEmptyData
	}

	var inBlob windows.DataBlob
	inBlob.Size = uint32(len(plaintext))
	inBlob.Data = &plaintext[0]

	var entropyBlob *windows.DataBlob
	if len(v.entropy) > 0 {
		entropyBlob = &windows.DataBlob{
			Size: uint32(len(v.entropy)),
			Data: &v.entropy[0],
		}
	}

	var outBlob windows.DataBlob
	// Flags = 0 enforces current user scope, strictly forbidding CRYPTPROTECT_LOCAL_MACHINE.
	err := windows.CryptProtectData(&inBlob, nil, entropyBlob, 0, nil, 0, &outBlob)
	if err != nil {
		return nil, fmt.Errorf("vault: DPAPI protect failed: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(outBlob.Data)))

	ciphertext := make([]byte, outBlob.Size)
	copy(ciphertext, unsafe.Slice(outBlob.Data, outBlob.Size))
	return ciphertext, nil
}

// Decrypt decrypts DPAPI-encrypted ciphertext using CryptUnprotectData.
func (v *DPAPIVault) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, ErrEmptyData
	}

	var inBlob windows.DataBlob
	inBlob.Size = uint32(len(ciphertext))
	inBlob.Data = &ciphertext[0]

	var entropyBlob *windows.DataBlob
	if len(v.entropy) > 0 {
		entropyBlob = &windows.DataBlob{
			Size: uint32(len(v.entropy)),
			Data: &v.entropy[0],
		}
	}

	var outBlob windows.DataBlob
	err := windows.CryptUnprotectData(&inBlob, nil, entropyBlob, 0, nil, 0, &outBlob)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(outBlob.Data)))

	plaintext := make([]byte, outBlob.Size)
	copy(plaintext, unsafe.Slice(outBlob.Data, outBlob.Size))
	return plaintext, nil
}
