package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"media-fetcher/pkg/cookiejar"
	"media-fetcher/pkg/vault"
)

var (
	ErrAccountNotFound = errors.New("auth: account not found")
	ErrNoActiveAccount = errors.New("auth: no active account for site")
)

// CredentialService manages site accounts, session life-cycles, cryptographic persistence,
// and strictly isolated HTTP clients.
type CredentialService struct {
	mu         sync.RWMutex
	storageDir string
	vault      vault.Vault
	accounts   map[string]*Account                       // key: siteID + ":" + accountID
	jars       map[string]*cookiejar.PersistentCookieJar // key: siteID + ":" + accountID
}

// NewCredentialService initializes the credential manager with the provided vault and directory.
func NewCredentialService(storageDir string, v vault.Vault) (*CredentialService, error) {
	if storageDir == "" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			storageDir = filepath.Join(appData, "media-fetcher", "credentials")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("auth: cannot determine home dir: %w", err)
			}
			storageDir = filepath.Join(home, ".media-fetcher", "credentials")
		}
	}

	if err := os.MkdirAll(storageDir, 0700); err != nil {
		return nil, fmt.Errorf("auth: failed to create storage dir: %w", err)
	}

	if v == nil {
		v = vault.NewDefaultVault()
	}

	s := &CredentialService{
		storageDir: storageDir,
		vault:      v,
		accounts:   make(map[string]*Account),
		jars:       make(map[string]*cookiejar.PersistentCookieJar),
	}

	if err := s.loadAccountsMeta(); err != nil {
		// Log or handle initial metadata load; start fresh if none exists
	}

	// Restore jars for loaded accounts
	for key, acc := range s.accounts {
		credFile := s.credFilePath(acc.SiteID, acc.AccountID)
		if _, err := os.Stat(credFile); err == nil {
			jar := cookiejar.New(acc.SiteID, acc.AccountID)
			if err := jar.Load(credFile, s.vault); err == nil {
				s.jars[key] = jar
			} else {
				// Decryption failed or file corrupted -> mark status expired/unverified without crashing
				acc.Status = StatusExpired
			}
		}
	}

	return s, nil
}

func accountKey(siteID, accountID string) string {
	return strings.ToLower(siteID) + ":" + accountID
}

func (s *CredentialService) credFilePath(siteID, accountID string) string {
	// Sanitize filename to prevent directory traversal
	safeSite := strings.ReplaceAll(siteID, "/", "_")
	safeAccount := strings.ReplaceAll(accountID, "/", "_")
	return filepath.Join(s.storageDir, fmt.Sprintf("%s_%s.cred", safeSite, safeAccount))
}

func (s *CredentialService) metaFilePath() string {
	return filepath.Join(s.storageDir, "accounts_meta.json")
}

func (s *CredentialService) loadAccountsMeta() error {
	path := s.metaFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var list []*Account
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	for _, a := range list {
		s.accounts[accountKey(a.SiteID, a.AccountID)] = a
	}
	return nil
}

func (s *CredentialService) saveAccountsMetaLocked() error {
	var list []*Account
	for _, a := range s.accounts {
		list = append(list, a)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.metaFilePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	_ = os.Remove(s.metaFilePath())
	return os.Rename(tmp, s.metaFilePath())
}

// RegisterOrUpdateAccount registers an account with its corresponding CookieJar,
// encrypts and saves the jar to disk, and updates the accounts index.
func (s *CredentialService) RegisterOrUpdateAccount(acc *Account, jar *cookiejar.PersistentCookieJar) error {
	if acc == nil || acc.SiteID == "" || acc.AccountID == "" {
		return errors.New("auth: invalid account payload")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := accountKey(acc.SiteID, acc.AccountID)

	// Determine if this should be active (if first account for this site)
	hasActive := false
	for _, existing := range s.accounts {
		if existing.SiteID == acc.SiteID && existing.IsActive && existing.AccountID != acc.AccountID {
			hasActive = true
			break
		}
	}
	if !hasActive {
		acc.IsActive = true
	}

	if acc.CreatedAt.IsZero() {
		acc.CreatedAt = time.Now()
	}
	acc.LastValidatedAt = time.Now()
	acc.Status = StatusActive

	// Persist encrypted jar
	if jar != nil {
		credPath := s.credFilePath(acc.SiteID, acc.AccountID)
		if err := jar.Save(credPath, s.vault); err != nil {
			return fmt.Errorf("auth: failed to persist credentials: %w", err)
		}
		s.jars[key] = jar
	}

	s.accounts[key] = acc
	return s.saveAccountsMetaLocked()
}

// GetAccount retrieves an account by SiteID and AccountID.
func (s *CredentialService) GetAccount(siteID, accountID string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acc, ok := s.accounts[accountKey(siteID, accountID)]
	if !ok {
		return nil, ErrAccountNotFound
	}
	return acc, nil
}

// GetActiveAccount returns the currently active account for a site.
func (s *CredentialService) GetActiveAccount(siteID string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, acc := range s.accounts {
		if strings.EqualFold(acc.SiteID, siteID) && acc.IsActive {
			return acc, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrNoActiveAccount, siteID)
}

// SetActiveAccount switches the active account for a specific site.
func (s *CredentialService) SetActiveAccount(siteID, accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	targetKey := accountKey(siteID, accountID)
	if _, ok := s.accounts[targetKey]; !ok {
		return ErrAccountNotFound
	}

	for key, acc := range s.accounts {
		if strings.EqualFold(acc.SiteID, siteID) {
			acc.IsActive = (key == targetKey)
		}
	}

	return s.saveAccountsMetaLocked()
}

// ListAccounts returns sanitized accounts for UI display for a specific site (or all sites if empty).
func (s *CredentialService) ListAccounts(siteID string) []SafeAccount {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []SafeAccount
	for _, acc := range s.accounts {
		if siteID == "" || strings.EqualFold(acc.SiteID, siteID) {
			result = append(result, acc.ToSafe())
		}
	}
	return result
}

// GetClient returns a dedicated http.Client bound to the specific account's PersistentCookieJar.
func (s *CredentialService) GetClient(siteID, accountID string) (*http.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := accountKey(siteID, accountID)
	jar, ok := s.jars[key]
	if !ok {
		return nil, fmt.Errorf("%w: no jar found for %s", ErrAccountNotFound, key)
	}

	return &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}, nil
}

// GetAnonymousClient returns a completely isolated, clean http.Client without any stored credentials.
// It is guaranteed never to leak or use any account's saved cookies.
func (s *CredentialService) GetAnonymousClient(siteID string) *http.Client {
	return &http.Client{
		// Fresh ephemeral in-memory jar, completely isolated from saved credentials
		Jar:     cookiejar.New(siteID, "anonymous"),
		Timeout: 30 * time.Second,
	}
}

// DeleteAccount permanently purges the account's encrypted credentials from disk
// and destroys its in-memory session and jar.
func (s *CredentialService) DeleteAccount(siteID, accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := accountKey(siteID, accountID)
	acc, ok := s.accounts[key]
	if !ok {
		return ErrAccountNotFound
	}

	// 1. Clear memory jar
	if jar, exists := s.jars[key]; exists {
		jar.Clear()
		delete(s.jars, key)
	}

	// 2. Remove encrypted credential file
	credPath := s.credFilePath(siteID, accountID)
	_ = os.Remove(credPath)

	// 3. Update active account if needed
	wasActive := acc.IsActive
	delete(s.accounts, key)

	if wasActive {
		for _, remaining := range s.accounts {
			if strings.EqualFold(remaining.SiteID, siteID) {
				remaining.IsActive = true
				break
			}
		}
	}

	return s.saveAccountsMetaLocked()
}

// ValidateAccount runs an external validator func against the account's client,
// updating account verification status and last validated timestamp.
func (s *CredentialService) ValidateAccount(
	ctx context.Context,
	siteID, accountID string,
	validator func(ctx context.Context, client *http.Client) (name, avatar string, valid bool, meta map[string]string, err error),
) error {
	client, err := s.GetClient(siteID, accountID)
	if err != nil {
		return err
	}

	name, avatar, valid, meta, err := validator(ctx, client)

	s.mu.Lock()
	defer s.mu.Unlock()

	key := accountKey(siteID, accountID)
	acc, ok := s.accounts[key]
	if !ok {
		return ErrAccountNotFound
	}

	acc.LastValidatedAt = time.Now()
	if err != nil || !valid {
		acc.Status = StatusExpired
	} else {
		acc.Status = StatusActive
		if name != "" {
			acc.Name = name
		}
		if avatar != "" {
			acc.AvatarURL = avatar
		}
		if len(meta) > 0 {
			acc.Metadata = meta
		}
	}

	return s.saveAccountsMetaLocked()
}
