package auth

import (
	"net/http"
	"net/url"
	"os"
	"testing"

	"media-fetcher/pkg/cookiejar"
	"media-fetcher/pkg/vault"
)

func TestCredentialService_IsolationAndPersistence(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("test_user")

	service, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatalf("failed to init service: %v", err)
	}

	biliURL, _ := url.Parse("https://bilibili.com")

	// Register Account A
	jarA := cookiejar.New("bilibili", "user_a")
	jarA.SetCookies(biliURL, []*http.Cookie{
		{Name: "SESSDATA", Value: "token_for_user_a", Domain: ".bilibili.com", Path: "/"},
	})
	accA := &Account{
		SiteID:    "bilibili",
		AccountID: "user_a",
		Name:      "Alice",
	}
	if err := service.RegisterOrUpdateAccount(accA, jarA); err != nil {
		t.Fatalf("register Alice failed: %v", err)
	}

	// Register Account B
	jarB := cookiejar.New("bilibili", "user_b")
	jarB.SetCookies(biliURL, []*http.Cookie{
		{Name: "SESSDATA", Value: "token_for_user_b", Domain: ".bilibili.com", Path: "/"},
	})
	accB := &Account{
		SiteID:    "bilibili",
		AccountID: "user_b",
		Name:      "Bob",
	}
	if err := service.RegisterOrUpdateAccount(accB, jarB); err != nil {
		t.Fatalf("register Bob failed: %v", err)
	}

	// 1. Verify Client Isolation
	clientA, err := service.GetClient("bilibili", "user_a")
	if err != nil {
		t.Fatalf("get client A: %v", err)
	}
	cookiesA := clientA.Jar.Cookies(biliURL)
	if len(cookiesA) != 1 || cookiesA[0].Value != "token_for_user_a" {
		t.Fatalf("unexpected cookies for A: %v", cookiesA)
	}

	clientB, err := service.GetClient("bilibili", "user_b")
	if err != nil {
		t.Fatalf("get client B: %v", err)
	}
	cookiesB := clientB.Jar.Cookies(biliURL)
	if len(cookiesB) != 1 || cookiesB[0].Value != "token_for_user_b" {
		t.Fatalf("unexpected cookies for B: %v", cookiesB)
	}

	// 2. Verify Anonymous Client Isolation (Must be completely empty!)
	anonClient := service.GetAnonymousClient("bilibili")
	anonCookies := anonClient.Jar.Cookies(biliURL)
	if len(anonCookies) != 0 {
		t.Fatalf("anonymous client must NOT have any cookies, got: %v", anonCookies)
	}

	// 3. Verify Active Account Switching
	active, err := service.GetActiveAccount("bilibili")
	if err != nil || active.AccountID != "user_a" {
		t.Fatalf("expected Alice to be active initially, got: %+v", active)
	}

	if err := service.SetActiveAccount("bilibili", "user_b"); err != nil {
		t.Fatalf("set active account failed: %v", err)
	}
	active, err = service.GetActiveAccount("bilibili")
	if err != nil || active.AccountID != "user_b" {
		t.Fatalf("expected Bob to be active, got: %+v", active)
	}

	// 4. Verify Restart & Persistence
	serviceRestarted, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatalf("restart service failed: %v", err)
	}

	restoredB, err := serviceRestarted.GetClient("bilibili", "user_b")
	if err != nil {
		t.Fatalf("restored get client B: %v", err)
	}
	restoredCookiesB := restoredB.Jar.Cookies(biliURL)
	if len(restoredCookiesB) != 1 || restoredCookiesB[0].Value != "token_for_user_b" {
		t.Fatalf("restored cookies for B mismatch: %v", restoredCookiesB)
	}

	// 5. Verify Account Deletion
	credFileA := service.credFilePath("bilibili", "user_a")
	if _, err := os.Stat(credFileA); err != nil {
		t.Fatalf("credential file for A should exist before delete: %v", err)
	}

	if err := service.DeleteAccount("bilibili", "user_a"); err != nil {
		t.Fatalf("delete account A failed: %v", err)
	}

	// Encrypted file must be physically removed
	if _, err := os.Stat(credFileA); !os.IsNotExist(err) {
		t.Fatalf("credential file for A should be deleted, got: %v", err)
	}

	// Should not be able to get client A anymore
	if _, err := service.GetClient("bilibili", "user_a"); err == nil {
		t.Fatal("expected error getting deleted account client")
	}
}

func TestCredentialService_CorruptFileHandling(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("test_user")

	service, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	jar := cookiejar.New("bilibili", "corrupt_user")
	acc := &Account{SiteID: "bilibili", AccountID: "corrupt_user", Name: "Corrupt"}
	if err := service.RegisterOrUpdateAccount(acc, jar); err != nil {
		t.Fatal(err)
	}

	// Deliberately corrupt the encrypted credential file
	credFile := service.credFilePath("bilibili", "corrupt_user")
	_ = os.WriteFile(credFile, []byte("totally_corrupt_data"), 0600)

	// Restart service -> must not panic, should mark status expired
	restarted, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatalf("service restart with corrupt file should not error out: %v", err)
	}

	restoredAcc, err := restarted.GetAccount("bilibili", "corrupt_user")
	if err != nil {
		t.Fatal(err)
	}
	if restoredAcc.Status != StatusExpired {
		t.Fatalf("expected status expired for corrupt file, got: %s", restoredAcc.Status)
	}
}
