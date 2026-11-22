package cookiejar

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"media-fetcher/pkg/vault"
)

func parseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestCookieJar_DomainAndHostOnly(t *testing.T) {
	jar := New("bilibili", "user1")

	// 1. HostOnly cookie (no domain specified)
	apiURL := parseURL("https://api.bilibili.com/x/web-interface/nav")
	jar.SetCookies(apiURL, []*http.Cookie{
		{Name: "host_token", Value: "val_host", Path: "/"},
	})

	// 2. Domain cookie (domain = .bilibili.com)
	jar.SetCookies(apiURL, []*http.Cookie{
		{Name: "global_token", Value: "val_global", Domain: ".bilibili.com", Path: "/"},
	})

	// 3. Rejected domain cookie (domain mismatch)
	jar.SetCookies(apiURL, []*http.Cookie{
		{Name: "evil_token", Value: "val_evil", Domain: "google.com", Path: "/"},
	})

	// Check on api.bilibili.com -> should get host_token and global_token, but not evil_token
	cookies := jar.Cookies(apiURL)
	cookieMap := make(map[string]string)
	for _, c := range cookies {
		cookieMap[c.Name] = c.Value
	}

	if cookieMap["host_token"] != "val_host" {
		t.Errorf("expected host_token on api.bilibili.com, got %v", cookieMap)
	}
	if cookieMap["global_token"] != "val_global" {
		t.Errorf("expected global_token on api.bilibili.com, got %v", cookieMap)
	}
	if _, ok := cookieMap["evil_token"]; ok {
		t.Errorf("evil_token should have been rejected for domain mismatch")
	}

	// Check on www.bilibili.com -> should get global_token, but NOT host_token
	wwwURL := parseURL("https://www.bilibili.com/video/BV123")
	wwwCookies := jar.Cookies(wwwURL)
	wwwMap := make(map[string]string)
	for _, c := range wwwCookies {
		wwwMap[c.Name] = c.Value
	}

	if _, ok := wwwMap["host_token"]; ok {
		t.Errorf("host_token should NOT be sent to www.bilibili.com")
	}
	if wwwMap["global_token"] != "val_global" {
		t.Errorf("expected global_token on www.bilibili.com, got %v", wwwMap)
	}
}

func TestCookieJar_PathMatching(t *testing.T) {
	jar := New("testsite", "user1")
	baseURL := parseURL("https://example.com/api/v1/user")

	jar.SetCookies(baseURL, []*http.Cookie{
		{Name: "root_cookie", Value: "1", Path: "/"},
		{Name: "api_cookie", Value: "2", Path: "/api"},
		{Name: "v1_cookie", Value: "3", Path: "/api/v1"},
		{Name: "v2_cookie", Value: "4", Path: "/api/v2"},
	})

	// Request to /api/v1/user should match root_cookie, api_cookie, v1_cookie, but NOT v2_cookie
	cookies := jar.Cookies(baseURL)
	m := make(map[string]string)
	for _, c := range cookies {
		m[c.Name] = c.Value
	}

	if m["root_cookie"] != "1" || m["api_cookie"] != "2" || m["v1_cookie"] != "3" {
		t.Errorf("missing expected cookies: %v", m)
	}
	if _, ok := m["v2_cookie"]; ok {
		t.Errorf("v2_cookie should not match /api/v1/user: %v", m)
	}

	// Request to /api-doc should NOT match /api
	apiDocURL := parseURL("https://example.com/api-doc")
	docCookies := jar.Cookies(apiDocURL)
	docMap := make(map[string]string)
	for _, c := range docCookies {
		docMap[c.Name] = c.Value
	}
	if _, ok := docMap["api_cookie"]; ok {
		t.Errorf("api_cookie should not match /api-doc: %v", docMap)
	}
}

func TestCookieJar_SecureFlag(t *testing.T) {
	jar := New("testsite", "user1")
	httpsURL := parseURL("https://example.com/test")
	httpURL := parseURL("http://example.com/test")

	jar.SetCookies(httpsURL, []*http.Cookie{
		{Name: "sec_cookie", Value: "secret", Secure: true, Path: "/"},
		{Name: "plain_cookie", Value: "normal", Secure: false, Path: "/"},
	})

	// HTTPS request should receive both
	httpsCookies := jar.Cookies(httpsURL)
	if len(httpsCookies) != 2 {
		t.Fatalf("expected 2 cookies on https, got %d", len(httpsCookies))
	}

	// HTTP request should only receive plain_cookie
	httpCookies := jar.Cookies(httpURL)
	if len(httpCookies) != 1 || httpCookies[0].Name != "plain_cookie" {
		t.Fatalf("expected only plain_cookie on http, got %v", httpCookies)
	}
}

func TestCookieJar_ExpirationAndDeletion(t *testing.T) {
	jar := New("testsite", "user1")
	u := parseURL("https://example.com/")

	// Set a normal cookie
	jar.SetCookies(u, []*http.Cookie{
		{Name: "temp", Value: "to_be_deleted", Path: "/"},
		{Name: "already_expired", Value: "expired", Path: "/", Expires: time.Now().Add(-1 * time.Hour)},
	})

	// already_expired should not be returned
	cookies := jar.Cookies(u)
	if len(cookies) != 1 || cookies[0].Name != "temp" {
		t.Fatalf("expected only 'temp' cookie, got %v", cookies)
	}

	// Delete 'temp' via MaxAge < 0
	jar.SetCookies(u, []*http.Cookie{
		{Name: "temp", Value: "", Path: "/", MaxAge: -1},
	})

	cookies = jar.Cookies(u)
	if len(cookies) != 0 {
		t.Fatalf("expected 0 cookies after deletion, got %v", cookies)
	}
}

func TestCookieJar_SaveAndLoadWithVault(t *testing.T) {
	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "credentials", "bilibili_user1.bin")

	v := vault.NewMockVault("user1")
	jar1 := New("bilibili", "user1")

	u := parseURL("https://www.bilibili.com/")
	jar1.SetCookies(u, []*http.Cookie{
		{
			Name:     "SESSDATA",
			Value:    "sess_secret_value",
			Domain:   ".bilibili.com",
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(24 * time.Hour),
		},
		{
			Name:    "expired_cookie",
			Value:   "will_not_persist",
			Domain:  ".bilibili.com",
			Path:    "/",
			Expires: time.Now().Add(-10 * time.Minute),
		},
	})

	// Save to disk
	if err := jar1.Save(savePath, v); err != nil {
		t.Fatalf("jar Save failed: %v", err)
	}

	// Verify file is encrypted on disk (should not contain plaintext 'sess_secret_value')
	rawBytes, err := os.ReadFile(savePath)
	if err != nil {
		t.Fatalf("read saved file failed: %v", err)
	}
	if string(rawBytes) == "" || string(rawBytes) == "sess_secret_value" {
		t.Fatal("saved file should be encrypted ciphertext")
	}

	// Load into a new jar
	jar2 := New("bilibili", "user1")
	if err := jar2.Load(savePath, v); err != nil {
		t.Fatalf("jar Load failed: %v", err)
	}

	loadedCookies := jar2.Cookies(u)
	if len(loadedCookies) != 1 {
		t.Fatalf("expected 1 cookie restored, got %d", len(loadedCookies))
	}
	c := loadedCookies[0]
	if c.Name != "SESSDATA" || c.Value != "sess_secret_value" {
		t.Errorf("cookie data mismatch: %+v", c)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie flags mismatch: HttpOnly=%v, Secure=%v, SameSite=%v", c.HttpOnly, c.Secure, c.SameSite)
	}

	// Account mismatch protection: loading user1 data into user2 jar must fail
	jarMismatch := New("bilibili", "user2")
	if err := jarMismatch.Load(savePath, v); !errors.Is(err, ErrAccountMismatch) {
		t.Fatalf("expected ErrAccountMismatch, got %v", err)
	}
}
