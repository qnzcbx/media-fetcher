package test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"media-fetcher/core/auth"
	"media-fetcher/pkg/cookiejar"
	"media-fetcher/pkg/vault"
)

func TestAdversarial_CookieJarRevokeRace(t *testing.T) {
	jar := cookiejar.New("bilibili", "test_user")
	v := vault.NewDefaultVault()
	jar.Save(t.TempDir()+"/test.cred", v)

	var wg sync.WaitGroup

	// Simulate rapid cookie updates
	wg.Add(1)
	go func() {
		defer wg.Done()
		u, _ := url.Parse("https://bilibili.com")
		for i := 0; i < 100; i++ {
			jar.SetCookies(u, []*http.Cookie{
				{Name: "SESSDATA", Value: "val"},
			})
			time.Sleep(time.Millisecond)
		}
	}()

	// Simulate concurrent revoke
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		jar.Revoke()
	}()

	wg.Wait()
	if !jar.IsRevoked() {
		t.Fatal("jar should be revoked")
	}
}

func TestAdversarial_DeleteAccountCancelsRequest(t *testing.T) {
	// A server that hangs until context is canceled
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	storageDir := t.TempDir()
	v := vault.NewDefaultVault()
	srv, err := auth.NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatalf("failed to create auth service: %v", err)
	}

	jar := cookiejar.New("bilibili", "test_user")
	acc := &auth.Account{SiteID: "bilibili", AccountID: "test_user"}
	if err := srv.RegisterOrUpdateAccount(acc, jar); err != nil {
		t.Fatalf("failed to register account: %v", err)
	}

	client, err := srv.GetClient("bilibili", "test_user")
	if err != nil {
		t.Fatalf("failed to get client: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		req, _ := http.NewRequest("GET", server.URL, nil)
		_, err := client.Do(req)
		errCh <- err
	}()

	// Give the request some time to reach the server
	time.Sleep(200 * time.Millisecond)

	// Delete the account, which revokes the jar
	if err := srv.DeleteAccount("bilibili", "test_user"); err != nil {
		t.Fatalf("failed to delete account: %v", err)
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected request to fail due to context cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("request did not cancel within timeout after account deletion")
	}
}
