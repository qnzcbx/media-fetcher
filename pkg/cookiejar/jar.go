package cookiejar

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"media-fetcher/pkg/vault"
)

var (
	ErrInvalidSchemaVersion = errors.New("cookiejar: unsupported schema version")
	ErrAccountMismatch      = errors.New("cookiejar: site_id or account_id mismatch")
)

// PersistentCookieJar implements http.CookieJar with explicit RFC 6265 state tracking,
// versioned serialization, atomic file updates, and encryption via CredentialVault.
type PersistentCookieJar struct {
	mu        sync.RWMutex
	siteID    string
	accountID string
	entries   map[string]CookieEntry // keyed by CookieEntry.Key()
}

// New creates a new in-memory PersistentCookieJar for a specific SiteID and AccountID.
func New(siteID, accountID string) *PersistentCookieJar {
	return &PersistentCookieJar{
		siteID:    siteID,
		accountID: accountID,
		entries:   make(map[string]CookieEntry),
	}
}

// SiteID returns the site identifier.
func (j *PersistentCookieJar) SiteID() string {
	return j.siteID
}

// AccountID returns the account identifier.
func (j *PersistentCookieJar) AccountID() string {
	return j.accountID
}

// SetCookies handles the Set-Cookie headers received from an HTTP response for url u.
func (j *PersistentCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if u == nil || len(cookies) == 0 {
		return
	}

	now := time.Now()
	host := strings.ToLower(u.Hostname())

	j.mu.Lock()
	defer j.mu.Unlock()

	for _, c := range cookies {
		if c == nil || c.Name == "" {
			continue
		}

		domain := strings.ToLower(c.Domain)
		hostOnly := false

		if domain == "" {
			domain = host
			hostOnly = true
		} else {
			// Strip leading dot per RFC 6265
			domain = strings.TrimPrefix(domain, ".")
			// Security validation: host must domain-match specified domain
			if !domainMatch(host, domain, false) {
				// Rejected: cannot set cookie for unrelated domain
				continue
			}
		}

		// Determine path
		cookiePath := c.Path
		if cookiePath == "" || cookiePath[0] != '/' {
			cookiePath = defaultPath(u.Path)
		}

		// Determine expiration
		hasExpires := false
		var expires time.Time
		isDelete := false

		if c.MaxAge < 0 {
			isDelete = true
		} else if c.MaxAge > 0 {
			hasExpires = true
			expires = now.Add(time.Duration(c.MaxAge) * time.Second)
		} else if !c.Expires.IsZero() {
			hasExpires = true
			expires = c.Expires
			if expires.Before(now) || expires.Equal(now) {
				isDelete = true
			}
		}

		sameSiteStr := "Default"
		switch c.SameSite {
		case http.SameSiteLaxMode:
			sameSiteStr = "Lax"
		case http.SameSiteStrictMode:
			sameSiteStr = "Strict"
		case http.SameSiteNoneMode:
			sameSiteStr = "None"
		}

		entry := CookieEntry{
			Name:           c.Name,
			Value:          c.Value,
			Domain:         domain,
			Path:           cookiePath,
			HostOnly:       hostOnly,
			Secure:         c.Secure,
			HttpOnly:       c.HttpOnly,
			SameSite:       sameSiteStr,
			CreationTime:   now,
			LastAccessTime: now,
			Expires:        expires,
			HasExpires:     hasExpires,
			MaxAge:         c.MaxAge,
		}

		key := entry.Key()
		if isDelete {
			delete(j.entries, key)
		} else {
			j.entries[key] = entry
		}
	}
}

// Cookies returns the unexpired cookies that match url u.
func (j *PersistentCookieJar) Cookies(u *url.URL) []*http.Cookie {
	if u == nil {
		return nil
	}

	now := time.Now()
	j.mu.Lock()
	defer j.mu.Unlock()

	var matched []*http.Cookie
	var expiredKeys []string

	for key, entry := range j.entries {
		if entry.IsExpired(now) {
			expiredKeys = append(expiredKeys, key)
			continue
		}
		if entry.MatchURL(u, now) {
			// Update last access time
			entry.LastAccessTime = now
			j.entries[key] = entry
			matched = append(matched, entry.ToHTTPCookie())
		}
	}

	// Clean up expired cookies encountered
	for _, k := range expiredKeys {
		delete(j.entries, k)
	}

	return matched
}

// AllEntries returns a snapshot of all active, non-expired cookie entries.
func (j *PersistentCookieJar) AllEntries() []CookieEntry {
	j.mu.RLock()
	defer j.mu.RUnlock()

	now := time.Now()
	var list []CookieEntry
	for _, e := range j.entries {
		if !e.IsExpired(now) {
			list = append(list, e)
		}
	}
	return list
}

// Clear removes all cookie entries from memory.
func (j *PersistentCookieJar) Clear() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.entries = make(map[string]CookieEntry)
}

// Save serializes active cookies to a versioned JSON schema, encrypts it with Vault,
// and writes it atomically to disk via temporary file rename.
func (j *PersistentCookieJar) Save(filePath string, v vault.Vault) error {
	if v == nil {
		return errors.New("cookiejar: vault is nil")
	}

	j.mu.RLock()
	now := time.Now()
	var validCookies []CookieEntry
	for _, e := range j.entries {
		if !e.IsExpired(now) {
			validCookies = append(validCookies, e)
		}
	}
	schema := StorageSchema{
		Version:   CurrentSchemaVersion,
		SiteID:    j.siteID,
		AccountID: j.accountID,
		UpdatedAt: now,
		Cookies:   validCookies,
	}
	j.mu.RUnlock()

	plaintext, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("cookiejar: marshal schema failed: %w", err)
	}

	ciphertext, err := v.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("cookiejar: encrypt failed: %w", err)
	}

	// Ensure destination directory exists with safe permissions
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cookiejar: mkdir failed: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tmpPath := filePath + fmt.Sprintf(".tmp.%d", time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, ciphertext, 0600); err != nil {
		return fmt.Errorf("cookiejar: write temp failed: %w", err)
	}

	// On Windows, if destination exists, os.Rename might fail if target cannot be replaced cleanly
	// In Go 1.21+, os.Rename uses MoveFileExW with MOVEFILE_REPLACE_EXISTING, but removing it first
	// provides extra safety across edge cases.
	if err := atomicReplaceFile(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("cookiejar: atomic replace failed: %w", err)
	}

	return nil
}

// Load reads encrypted file from disk, decrypts with Vault, validates schema version,
// filters out expired cookies, and restores active cookies into the jar.
func (j *PersistentCookieJar) Load(filePath string, v vault.Vault) error {
	if v == nil {
		return errors.New("cookiejar: vault is nil")
	}

	ciphertext, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cookiejar: read file failed: %w", err)
	}

	plaintext, err := v.Decrypt(ciphertext)
	if err != nil {
		return fmt.Errorf("cookiejar: decrypt failed: %w", err)
	}

	var schema StorageSchema
	if err := json.Unmarshal(plaintext, &schema); err != nil {
		return fmt.Errorf("cookiejar: unmarshal failed: %w", err)
	}

	if schema.Version != CurrentSchemaVersion {
		return fmt.Errorf("%w: got %d, expected %d", ErrInvalidSchemaVersion, schema.Version, CurrentSchemaVersion)
	}

	if schema.SiteID != j.siteID || schema.AccountID != j.accountID {
		return fmt.Errorf("%w: stored (%s/%s) vs jar (%s/%s)",
			ErrAccountMismatch, schema.SiteID, schema.AccountID, j.siteID, j.accountID)
	}

	now := time.Now()
	j.mu.Lock()
	defer j.mu.Unlock()

	j.entries = make(map[string]CookieEntry)
	for _, e := range schema.Cookies {
		if !e.IsExpired(now) {
			j.entries[e.Key()] = e
		}
	}

	return nil
}

func atomicReplaceFile(src, dst string) error {
	// If dst exists, attempt os.Rename. If that fails on Windows, try removing dst first.
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	_ = os.Remove(dst)
	return os.Rename(src, dst)
}
