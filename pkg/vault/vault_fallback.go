//go:build !windows

package vault

// NewDefaultVault 在非 Windows 平台返回安全占位 Vault，拒绝可逆弱加密落盘。
func NewDefaultVault() Vault {
	return &UnavailableVault{}
}
