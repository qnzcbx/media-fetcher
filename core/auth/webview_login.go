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
	ErrSessionNotFound = errors.New("webview: 未找到登录会话")
	ErrSessionExpired  = errors.New("webview: 登录会话已过期或关闭")
	ErrValidationFail  = errors.New("webview: 身份凭证核验失败")
)

// WebViewSession 表示完全隔离的临时 WebView 登录会话。
type WebViewSession struct {
	SessionID string    `json:"session_id"`
	SiteID    string    `json:"site_id"`
	LoginURL  string    `json:"login_url"`
	CreatedAt time.Time `json:"created_at"`
	IsClosed  bool      `json:"is_closed"`
}

// WebViewLoginCoordinator 管理独立且隔离的 WebView 登录会话生命周期。
// 确保临时 webview 中的 Cookie 绝不与默认存储混用，并在取消或完成时即刻安全清理。
type WebViewLoginCoordinator struct {
	mu          sync.Mutex
	credService *CredentialService
	sessions    map[string]*WebViewSession
	siteURLs    map[string]string // 官方登录入口端点
}

// NewWebViewLoginCoordinator 创建新的 WebView 登录协调器。
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

// StartSession 为指定站点创建独立的专属隔离登录会话。
func (c *WebViewLoginCoordinator) StartSession(siteID string) (*WebViewSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	loginURL, ok := c.siteURLs[strings.ToLower(siteID)]
	if !ok {
		return nil, fmt.Errorf("webview: 不支持登录的站点: %s", siteID)
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

// CompleteSession 接收从隔离 WebView 会话中提取的 Cookie 列表，
// 执行站点专属的在线身份核验，成功后经由 CredentialVault 加密落盘。
// 临时会话随即立即关闭并安全销毁。
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
	// 立即置为关闭状态防止重用
	sess.IsClosed = true
	delete(c.sessions, sessionID)
	c.mu.Unlock()

	if len(cookies) == 0 {
		return nil, errors.New("webview: 会话未提供任何 Cookie 凭据")
	}

	// 1. 放入临时 Jar 中执行身份验证
	tempJar := cookiejar.New(sess.SiteID, "temp_webview_"+sessionID)
	dummyURL, _ := url.Parse("https://" + sess.SiteID + ".com")
	tempJar.SetCookies(dummyURL, cookies)

	verifyClient := &http.Client{Jar: tempJar, Timeout: 20 * time.Second}
	name, avatar, accountID, valid, meta, err := validator(ctx, verifyClient)
	if err != nil || !valid {
		return nil, fmt.Errorf("%w: %v", ErrValidationFail, err)
	}

	// 2. 构建正式持久化 Jar 并加密落盘
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
		return nil, fmt.Errorf("webview: 持久化凭据失败: %w", err)
	}

	safe := acc.ToSafe()
	return &safe, nil
}

// CancelSession 立即终止会话并抹除所有关联跟踪信息。
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
