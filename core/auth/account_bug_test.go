package auth

import (
	"testing"

	"media-fetcher/pkg/cookiejar"
	"media-fetcher/pkg/vault"
)

func TestRegisterOrUpdateAccount_PointerBug(t *testing.T) {
	storage := t.TempDir()
	backing := vault.NewMockVault("test")
	service, err := NewCredentialService(storage, backing)
	if err != nil {
		t.Fatal(err)
	}

	acc := &Account{SiteID: "bilibili", AccountID: "test_user"}
	jar1 := cookiejar.New("bilibili", "test_user")
	if err := service.RegisterOrUpdateAccount(acc, jar1); err != nil {
		t.Fatal(err)
	}

	// 获取已保存账号的指针。
	savedAcc, err := service.GetAccount("bilibili", "test_user")
	if err != nil {
		t.Fatal(err)
	}

	if savedAcc.Status != StatusActive {
		t.Fatalf("expected StatusActive, got %s", savedAcc.Status)
	}

	// 使用新的 CookieJar 和同一个账号指针更新账号。
	jar2 := cookiejar.New("bilibili", "test_user")
	if err := service.RegisterOrUpdateAccount(savedAcc, jar2); err != nil {
		t.Fatal(err)
	}

	// 验证更新后的账号状态。
	updatedAcc, err := service.GetAccount("bilibili", "test_user")
	if err != nil {
		t.Fatal(err)
	}

	if updatedAcc.Status != StatusActive {
		t.Fatalf("bug exists: expected StatusActive, got %s", updatedAcc.Status)
	}
}
