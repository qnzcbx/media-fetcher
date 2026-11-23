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

	// Fetch the saved account pointer
	savedAcc, err := service.GetAccount("bilibili", "test_user")
	if err != nil {
		t.Fatal(err)
	}

	if savedAcc.Status != StatusActive {
		t.Fatalf("expected StatusActive, got %s", savedAcc.Status)
	}

	// Update the account using a NEW jar, but the SAME account pointer
	jar2 := cookiejar.New("bilibili", "test_user")
	if err := service.RegisterOrUpdateAccount(savedAcc, jar2); err != nil {
		t.Fatal(err)
	}

	// Verify the status
	updatedAcc, err := service.GetAccount("bilibili", "test_user")
	if err != nil {
		t.Fatal(err)
	}

	if updatedAcc.Status != StatusActive {
		t.Fatalf("bug exists: expected StatusActive, got %s", updatedAcc.Status)
	}
}
