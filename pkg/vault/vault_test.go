package vault

import (
	"bytes"
	"errors"
	"runtime"
	"testing"
)

// 测试测试替身 MockVault 的多用户隔离
func TestMockVault_UserIsolation(t *testing.T) {
	vaultAlice := NewMockVault("alice")
	vaultBob := NewMockVault("bob")

	secret := []byte("alice_super_secret_cookie")
	encrypted, err := vaultAlice.Encrypt(secret)
	if err != nil {
		t.Fatalf("Alice 加密失败: %v", err)
	}

	// Alice 能够解密自己的数据
	decrypted, err := vaultAlice.Decrypt(encrypted)
	if err != nil || !bytes.Equal(decrypted, secret) {
		t.Fatalf("Alice 解密失败: %v, got %s", err, string(decrypted))
	}

	// Bob 尝试解密 Alice 的数据必须失败
	_, err = vaultBob.Decrypt(encrypted)
	if err == nil {
		t.Fatal("Bob 绝不能解密 Alice 的数据")
	}
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("期望 ErrDecryptionFailed, 实际: %v", err)
	}
}

// 测试空数据与损坏载荷处理
func TestMockVault_EmptyAndCorrupt(t *testing.T) {
	v := NewMockVault("tester")

	// 空数据检查
	if _, err := v.Encrypt(nil); !errors.Is(err, ErrEmptyData) {
		t.Fatalf("nil 输入期望 ErrEmptyData, 实际 %v", err)
	}
	if _, err := v.Encrypt([]byte{}); !errors.Is(err, ErrEmptyData) {
		t.Fatalf("空字节期望 ErrEmptyData, 实际 %v", err)
	}
	if _, err := v.Decrypt([]byte{}); !errors.Is(err, ErrEmptyData) {
		t.Fatalf("解密空字节期望 ErrEmptyData, 实际 %v", err)
	}

	// 损坏数据检查
	if _, err := v.Decrypt([]byte("garbage_data_too_short")); err == nil {
		t.Fatal("损坏数据期望解密失败")
	}
}

// H5 回归测试：非 Windows 默认 Vault 或未就绪安全后端必须拒绝弱加密落盘
func TestH5_UnavailableVault_Rejection(t *testing.T) {
	// 直接实例化 UnavailableVault 验证安全断言
	var u Vault
	if runtime.GOOS != "windows" {
		u = NewDefaultVault()
	} else {
		// 在 Windows 单元测试中直接测试该类型契约
		u = &UnavailableVault{}
	}

	_, err := u.Encrypt([]byte("sensitive_sessdata"))
	if !errors.Is(err, ErrVaultNotSupported) {
		t.Fatalf("期望 ErrVaultNotSupported, 实际得到: %v", err)
	}

	_, err = u.Decrypt([]byte("some_data"))
	if !errors.Is(err, ErrVaultNotSupported) {
		t.Fatalf("期望 ErrVaultNotSupported, 实际得到: %v", err)
	}
}

// Windows 原生 DPAPI 往返与安全边界测试
func TestWindowsDPAPI_Roundtrip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("跳过非 Windows 平台的 DPAPI 原生测试")
	}

	v := NewDefaultVault()
	secret := []byte("sessdata=secret_token_12345; Path=/; Domain=.bilibili.com")

	ciphertext, err := v.Encrypt(secret)
	if err != nil {
		t.Fatalf("DPAPI 加密失败: %v", err)
	}
	if bytes.Equal(ciphertext, secret) {
		t.Fatal("密文绝不能等于明文")
	}

	plaintext, err := v.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("DPAPI 解密失败: %v", err)
	}
	if !bytes.Equal(plaintext, secret) {
		t.Fatalf("解密结果与原文不符: got %s, want %s", string(plaintext), string(secret))
	}

	// 测试附加熵盐
	entropy := []byte("media_fetcher_app_salt")
	vEntropy := NewDPAPIVault(entropy)
	cipherEntropy, err := vEntropy.Encrypt(secret)
	if err != nil {
		t.Fatalf("带熵 DPAPI 加密失败: %v", err)
	}
	plainEntropy, err := vEntropy.Decrypt(cipherEntropy)
	if err != nil || !bytes.Equal(plainEntropy, secret) {
		t.Fatalf("带熵 DPAPI 解密失败: %v", err)
	}

	// 错误熵盐解密必须失败
	vWrongEntropy := NewDPAPIVault([]byte("wrong_salt"))
	_, err = vWrongEntropy.Decrypt(cipherEntropy)
	if err == nil {
		t.Fatal("使用错误熵盐必须解密失败")
	}
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("期望 ErrDecryptionFailed, 实际: %v", err)
	}

	// 空数据检查
	if _, err := v.Encrypt([]byte{}); !errors.Is(err, ErrEmptyData) {
		t.Fatalf("期望 ErrEmptyData, 实际 %v", err)
	}
	if _, err := v.Decrypt([]byte{}); !errors.Is(err, ErrEmptyData) {
		t.Fatalf("期望 ErrEmptyData, 实际 %v", err)
	}

	// 篡改密文头部检查
	corrupt := append([]byte(nil), ciphertext...)
	if len(corrupt) > 0 {
		corrupt[0] ^= 0xFF
	}
	_, err = v.Decrypt(corrupt)
	if err == nil {
		t.Fatal("篡改密文必须导致解密失败")
	}
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("篡改密文期望 ErrDecryptionFailed, 实际 %v", err)
	}
}
