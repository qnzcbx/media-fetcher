//go:build windows

package vault

import (
	"fmt"
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DPAPIVault 使用 Windows 原生数据保护 API (DPAPI) 实现 Vault。
// 严格工作于当前 Windows 用户作用域，禁止启用机器范围的 CRYPTPROTECT_LOCAL_MACHINE。
type DPAPIVault struct {
	entropy []byte
}

// NewDPAPIVault 创建基于 Windows 原生 DPAPI 的保险库实例。
// 可传入可选的附加熵盐（optionalEntropy）增加纵深防御，或传入 nil 使用默认当前用户作用域。
func NewDPAPIVault(optionalEntropy []byte) *DPAPIVault {
	var ent []byte
	if len(optionalEntropy) > 0 {
		ent = make([]byte, len(optionalEntropy))
		copy(ent, optionalEntropy)
	}
	return &DPAPIVault{entropy: ent}
}

// NewDefaultVault 返回 Windows 平台的标准 Vault 实现。
func NewDefaultVault() Vault {
	return NewDPAPIVault(nil)
}

// Encrypt 使用 CryptProtectData 在当前用户上下文下加密数据。
// 明确禁止使用 CRYPTPROTECT_LOCAL_MACHINE (0x4)，同时设置 CRYPTPROTECT_UI_FORBIDDEN (0x1) 禁止弹出系统交互提示框。
func (v *DPAPIVault) Encrypt(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, ErrEmptyData
	}
	if len(plaintext) > math.MaxInt32 {
		return nil, fmt.Errorf("vault: 明文长度超过安全上限 (%d 字节)", len(plaintext))
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
	// Flags = CRYPTPROTECT_UI_FORBIDDEN (0x1)，严格限制为当前用户作用域且禁止弹出 UI。
	const flags = windows.CRYPTPROTECT_UI_FORBIDDEN
	err := windows.CryptProtectData(&inBlob, nil, entropyBlob, 0, nil, flags, &outBlob)
	if err != nil {
		return nil, fmt.Errorf("vault: Windows DPAPI 加密失败: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(outBlob.Data)))

	ciphertext := make([]byte, outBlob.Size)
	copy(ciphertext, unsafe.Slice(outBlob.Data, outBlob.Size))
	return ciphertext, nil
}

// Decrypt 使用 CryptUnprotectData 解密当前用户作用域的密文。
func (v *DPAPIVault) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, ErrEmptyData
	}
	if len(ciphertext) > math.MaxInt32 {
		return nil, fmt.Errorf("vault: 密文长度超过安全上限 (%d 字节)", len(ciphertext))
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
	const flags = windows.CRYPTPROTECT_UI_FORBIDDEN
	err := windows.CryptUnprotectData(&inBlob, nil, entropyBlob, 0, nil, flags, &outBlob)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(outBlob.Data)))

	plaintext := make([]byte, outBlob.Size)
	copy(plaintext, unsafe.Slice(outBlob.Data, outBlob.Size))
	return plaintext, nil
}
