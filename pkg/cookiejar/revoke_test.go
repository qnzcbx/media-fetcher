package cookiejar

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"media-fetcher/pkg/vault"
)

type blockedVault struct {
	vault.Vault
	started chan struct{}
	release chan struct{}
}

func (v blockedVault) Encrypt(data []byte) ([]byte, error) {
	close(v.started)
	<-v.release
	return v.Vault.Encrypt(data)
}

func TestSaveCannotPublishAfterRevoke(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "session.cred")
	v := blockedVault{vault.NewMockVault("test"), make(chan struct{}), make(chan struct{})}
	jar := New("bilibili", "test")
	u := &url.URL{Scheme: "https", Host: "api.bilibili.com"}
	jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: "fixture"}})
	result := make(chan error, 1)
	go func() { result <- jar.Save(filename, v) }()
	<-v.started
	jar.Revoke()
	close(v.release)
	if err := <-result; !errors.Is(err, ErrRevoked) {
		t.Fatalf("save after revoke: %v", err)
	}
	if _, err := os.Stat(filename); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("credential file: %v", err)
	}
}

func TestQueuedSavePreservesCookieDeletion(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "session.cred")
	backing := vault.NewMockVault("test")
	blocked := blockedVault{backing, make(chan struct{}), make(chan struct{})}
	jar := New("bilibili", "test")
	u := &url.URL{Scheme: "https", Host: "api.bilibili.com"}
	jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: "fixture"}})
	first := make(chan error, 1)
	go func() { first <- jar.Save(filename, blocked) }()
	<-blocked.started
	jar.SetCookies(u, []*http.Cookie{{Name: "session", MaxAge: -1}})
	second := make(chan error, 1)
	go func() { second <- jar.Save(filename, backing) }()
	close(blocked.release)
	for _, result := range []chan error{first, second} {
		if err := <-result; err != nil {
			t.Fatal(err)
		}
	}
	restored := New("bilibili", "test")
	if err := restored.Load(filename, backing); err != nil {
		t.Fatal(err)
	}
	if len(restored.Cookies(u)) != 0 {
		t.Fatal("deleted cookie recovered from disk")
	}
}
