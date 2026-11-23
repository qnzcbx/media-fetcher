package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"media-fetcher/pkg/cookiejar"
	"media-fetcher/pkg/vault"
)

func TestSessionResponseLifetime(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		name := "complete"
		if revoke {
			name = "logout"
		}
		t.Run(name, func(t *testing.T) {
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				w.(http.Flusher).Flush()
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				_, _ = io.WriteString(w, "first")
				w.(http.Flusher).Flush()
				_, _ = io.WriteString(w, "second")
			}))
			defer server.Close()
			service, err := NewCredentialService(t.TempDir(), vault.NewMockVault("test"))
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"a", "b"} {
				if err := service.RegisterOrUpdateAccount(&Account{SiteID: "bilibili", AccountID: id}, cookiejar.New("bilibili", id)); err != nil {
					t.Fatal(err)
				}
			}
			a, err := service.GetClient("bilibili", "a")
			if err != nil {
				t.Fatal(err)
			}
			b, err := service.GetClient("bilibili", "b")
			if err != nil {
				t.Fatal(err)
			}
			a.Timeout = 3 * time.Second
			b.Timeout = 3 * time.Second
			first, err := a.Get(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer first.Body.Close()
			second, err := a.Get(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer second.Body.Close()
			other, err := b.Get(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Body.Close()
			if revoke {
				if err := service.DeleteAccount("bilibili", "a"); err != nil {
					t.Fatal(err)
				}
				for _, resp := range []*http.Response{first, second} {
					if _, err := io.ReadAll(resp.Body); err == nil {
						t.Fatal("revoked response remained readable")
					}
				}
				if _, err := a.Get(server.URL); err == nil {
					t.Fatal("revoked client accepted new request")
				}
			}
			close(release)
			responses := []*http.Response{other}
			if !revoke {
				responses = append(responses, first, second)
			}
			for _, resp := range responses {
				body, err := io.ReadAll(resp.Body)
				if err != nil || string(body) != "firstsecond" {
					t.Fatalf("read body %q: %v", body, err)
				}
				if err := resp.Body.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestPreviousClientRevokedOnReplacement(t *testing.T) {
	storage := t.TempDir()
	backing := vault.NewMockVault("test")
	service, err := NewCredentialService(storage, backing)
	if err != nil {
		t.Fatal(err)
	}
	account := Account{SiteID: "bilibili", AccountID: "a"}
	u := &url.URL{Scheme: "https", Host: "api.bilibili.com"}
	register := func(value string) {
		jar := cookiejar.New(account.SiteID, account.AccountID)
		jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: value}})
		copy := account
		if err := service.RegisterOrUpdateAccount(&copy, jar); err != nil {
			t.Fatal(err)
		}
	}
	register("old")
	previous, err := service.GetClient(account.SiteID, account.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	register("new")
	previous.Jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: "stale"}})
	if len(previous.Jar.Cookies(u)) != 0 {
		t.Fatal("previous session still serves cookies")
	}
	restored, err := NewCredentialService(storage, backing)
	if err != nil {
		t.Fatal(err)
	}
	current, err := restored.GetClient(account.SiteID, account.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	cookies := current.Jar.Cookies(u)
	if len(cookies) != 1 || cookies[0].Value != "new" {
		t.Fatal("restart did not restore replacement")
	}
}
