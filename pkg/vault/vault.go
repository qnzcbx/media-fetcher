// Package vault 提供敏感凭证数据的加密存储能力。
package vault

import (
	"errors"
	"fmt"
)

var (
	// ErrEmptyData 当尝试加密或解密空字节切片时返回。
	ErrEmptyData = errors.New("vault: 数据不能为空")
	// ErrDecryptionFailed 当解密失败时返回（密钥无效、用户不匹配、密文被篡改）。
	ErrDecryptionFailed = errors.New("vault: 解密失败")
	// ErrCorruptCiphertext 当密文结构损坏或不完整时返回。
	ErrCorruptCiphertext = errors.New("vault: 密文载荷损坏")
	// ErrVaultNotSupported 表示当前操作系统平台或环境暂未实现安全的受信任密钥库（如非 Windows 平台未接入系统钥匙串）。
	ErrVaultNotSupported = errors.New("vault: 平台安全保险库不可用（V1 仅在 Windows 原生 DPAPI 下启用持久化凭证，非 Windows 暂未接入安全密钥库）")
)

// Vault 定义了敏感凭证加密与解密的抽象接口。
type Vault interface {
	// Encrypt 加密明文字节切片。
	Encrypt(plaintext []byte) ([]byte, error)
	// Decrypt 解密密文字节切片。
	Decrypt(ciphertext []byte) ([]byte, error)
}

// UnavailableVault 代表在当前平台缺少受信任系统安全密钥库时的安全占位实现。
// 依据任务规格要求，未接入系统安全密钥库前严格拒绝明文或可逆伪加密落盘，
// 强制报错以保护用户凭证不被明文泄露，同时保留匿名模式正常运行。
type UnavailableVault struct{}

// Encrypt 在不支持的平台或后端上拒绝加密并返回明确错误。
func (u *UnavailableVault) Encrypt(plaintext []byte) ([]byte, error) {
	return nil, ErrVaultNotSupported
}

// Decrypt 在不支持的平台或后端上拒绝解密并返回明确错误。
func (u *UnavailableVault) Decrypt(ciphertext []byte) ([]byte, error) {
	return nil, ErrVaultNotSupported
}

// MockVault 仅用于测试的内存替身，用于跨平台模拟用户隔离、密文损坏与解密失败。
// 严禁作为生产环境默认实现。
type MockVault struct {
	UserID    string
	FailOnDec bool
}

// NewMockVault 返回绑定特定测试用户 ID 的 MockVault 实例（仅供测试显式注入使用）。
func NewMockVault(userID string) *MockVault {
	return &MockVault{UserID: userID}
}

// Encrypt 在明文前附加测试用户前缀并进行异或，用于测试场景。
func (m *MockVault) Encrypt(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, ErrEmptyData
	}
	prefix := []byte(fmt.Sprintf("MOCK_VAULT_V1:%s:", m.UserID))
	out := make([]byte, len(prefix)+len(plaintext))
	copy(out, prefix)
	for i, b := range plaintext {
		out[len(prefix)+i] = b ^ 0x5A
	}
	return out, nil
}

// Decrypt 验证测试用户前缀并还原数据。
func (m *MockVault) Decrypt(ciphertext []byte) ([]byte, error) {
	if m.FailOnDec {
		return nil, ErrDecryptionFailed
	}
	if len(ciphertext) == 0 {
		return nil, ErrEmptyData
	}
	prefix := []byte(fmt.Sprintf("MOCK_VAULT_V1:%s:", m.UserID))
	if len(ciphertext) < len(prefix) || string(ciphertext[:len(prefix)]) != string(prefix) {
		return nil, fmt.Errorf("%w: 用户不匹配或测试头部损坏", ErrDecryptionFailed)
	}
	payload := ciphertext[len(prefix):]
	out := make([]byte, len(payload))
	for i, b := range payload {
		out[i] = b ^ 0x5A
	}
	return out, nil
}
