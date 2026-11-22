//go:build !windows

package vault

// NewDefaultVault returns a fallback mock vault for non-Windows platforms.
func NewDefaultVault() Vault {
	return NewMockVault("default_user")
}
