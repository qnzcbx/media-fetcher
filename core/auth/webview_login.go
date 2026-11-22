package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"media-fetcher/pkg/cookiejar"
)

var (
	ErrSessionNotFound = errors.New("webview: login session not found")
	ErrSessionExpired  = errors.New("webview: login session expired or closed")
	ErrValidationFail  = errors.New("webview: identity validation failed")
)

// WebViewSession represents an isolated temporary WebView login session.
type WebViewSession struct {
	SessionID string    `json:"session_id"`
	SiteID    string    `json:"site_id"`
	LoginURL  string    `json:"login_url"`
	CreatedAt time.Time `json:"created_at"`
	IsClosed  bool      `json:"is_closed"`
}

// WebViewLoginCoordinator manages the lifecycle of independent, isolated WebView login sessions.
// It ensures that temporary webview cookies are never shared with default storage and are cleaned up on cancellation.
type WebViewLoginCoordinator struct {
	mu          sync.Mutex
	credService *CredentialService
	sessions    map[string]*WebViewSession
	siteURLs    map[string]string // official login entry points
}

// NewWebViewLoginCoordinator creates a new coordinator.
func NewWebViewLoginCoordinator(credService *CredentialService) *WebViewLoginCoordinator {
	return &WebViewLoginCoordinator{
		credService: credService,
		sessions:    make(map[string]*WebViewSession),
		siteURLs: map[string]string{
			"bilibili": "https://passport.bilibili.com/login",
			"lofter":   "https://www.lofter.com/login",
		},
	}
}

func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// StartSession creates a dedicated, isolated login session for a site.
func (c *WebViewLoginCoordinator) StartSession(siteID string) (*WebViewSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	loginURL, ok := c.siteURLs[strings.ToLower(siteID)]
	if !ok {
		return nil, fmt.Errorf("webview: unsupported site for login: %s", siteID)
	}

	sessionID := generateSessionID()
	sess := &WebViewSession{
		SessionID: sessionID,
		SiteID:    siteID,
		LoginURL:  loginURL,
		CreatedAt: time.Now(),
		IsClosed:  false,
	}

	c.sessions[sessionID] = sess
	return sess, nil
}

// CompleteSession accepts cookies extracted from the isolated WebView session,
// runs the site-specific identity validator, and persists to CredentialVault upon success.
// The temporary session is immediately closed and wiped.
func (c *WebViewLoginCoordinator) CompleteSession(
	ctx context.Context,
	sessionID string,
	cookies []*http.Cookie,
	validator func(ctx context.Context, client *http.Client) (name, avatar, accountID string, valid bool, meta map[string]string, err error),
) (*SafeAccount, error) {
	c.mu.Lock()
	sess, ok := c.sessions[sessionID]
	if !ok {
		c.mu.Unlock()
		return nil, ErrSessionNotFound
	}
	if sess.IsClosed {
		c.mu.Unlock()
		return nil, ErrSessionExpired
	}
	// Mark closed immediately to prevent reuse
	sess.IsClosed = true
	delete(c.sessions, sessionID)
	c.mu.Unlock()

	if len(cookies) == 0 {
		return nil, errors.New("webview: no cookies provided from session")
	}

	// 1. Put into temporary jar for verification
	tempJar := cookiejar.New(sess.SiteID, "temp_webview_"+sessionID)
	dummyURL, _ := url.Parse("https://" + sess.SiteID + ".com")
	tempJar.SetCookies(dummyURL, cookies)

	verifyClient := &http.Client{Jar: tempJar, Timeout: 20 * time.Second}
	name, avatar, accountID, valid, meta, err := validator(ctx, verifyClient)
	if err != nil || !valid {
		return nil, fmt.Errorf("%w: %v", ErrValidationFail, err)
	}

	// 2. Build final persistent jar
	finalJar := cookiejar.New(sess.SiteID, accountID)
	finalJar.SetCookies(dummyURL, cookies)

	acc := &Account{
		SiteID:          sess.SiteID,
		AccountID:       accountID,
		Name:            name,
		AvatarURL:       avatar,
		Status:          StatusActive,
		CreatedAt:       time.Now(),
		LastValidatedAt: time.Now(),
		Metadata:        meta,
	}

	if err := c.credService.RegisterOrUpdateAccount(acc, finalJar); err != nil {
		return nil, fmt.Errorf("webview: failed to persist credentials: %w", err)
	}

	safe := acc.ToSafe()
	return &safe, nil
}

// CancelSession terminates the session immediately and wipes all associated tracking.
func (c *WebViewLoginCoordinator) CancelSession(sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	sess, ok := c.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}

	sess.IsClosed = true
	delete(c.sessions, sessionID)
	return nil
}
