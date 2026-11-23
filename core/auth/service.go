package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"media-fetcher/pkg/cookiejar"
	"media-fetcher/pkg/fsutil"
	"media-fetcher/pkg/vault"
)

var (
	// ErrAccountNotFound 表示请求的账号不存在。
	ErrAccountNotFound = errors.New("auth: 账号未找到")
	// ErrNoActiveAccount 表示指定站点当前没有处于活动状态的账号。
	ErrNoActiveAccount = errors.New("auth: 当前站点无活动账号")
	// ErrSessionRevoked 表示由于账号已注销退出，旧客户端发起的请求被主动撤销阻断。
	ErrSessionRevoked = errors.New("auth: 账号会话已注销，请求被撤销拒绝")
)

// CredentialService 统一管理各站点的用户账号、会话生命周期、加密存储与隔离的 HTTP 客户端。
type CredentialService struct {
	mu         sync.RWMutex
	storageDir string
	vault      vault.Vault
	accounts   map[string]*Account                       // 键: siteID + ":" + accountID
	jars       map[string]*cookiejar.PersistentCookieJar // 键: siteID + ":" + accountID
}

// isHostTrustedForSite 判断主机名是否处于指定站点的信任边界内。
func isHostTrustedForSite(siteID, host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	switch strings.ToLower(siteID) {
	case "bilibili":
		for _, d := range []string{"bilibili.com", "hdslb.com", "bilivideo.com", "bilivideo.cn", "bilibili.tv"} {
			if host == d || strings.HasSuffix(host, "."+d) {
				return true
			}
		}
	case "lofter":
		for _, d := range []string{"lofter.com", "126.net", "163.com"} {
			if host == d || strings.HasSuffix(host, "."+d) {
				return true
			}
		}
	default:
		return false
	}
	return false
}

// revocableTransport 是可撤销的 HTTP 传输层，当底层 Jar 被撤销（账号注销）时，
// 立即阻断正在或后续发起的 HTTP 请求，确保已注销的 Client 无法继续发起请求或写回状态。
type revocableTransport struct {
	base   http.RoundTripper
	jar    *cookiejar.PersistentCookieJar
	siteID string
}

func (rt *revocableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.jar != nil && rt.jar.IsRevoked() {
		return nil, ErrSessionRevoked
	}
	if rt.siteID != "" && (req.URL.Scheme != "https" && !isLoopback(req.URL.Hostname()) || !isHostTrustedForSite(rt.siteID, req.URL.Hostname())) {
		req = req.Clone(req.Context())
		req.Header.Del("Cookie")
		req.Header.Del("Authorization")
	}
	base := rt.base
	if base == nil {
		base = http.DefaultTransport
	}
	if rt.jar == nil {
		return base.RoundTrip(req)
	}

	ctx, cancel := context.WithCancelCause(req.Context())
	stop := context.AfterFunc(rt.jar.Context(), func() { cancel(ErrSessionRevoked) })
	cleanup := func() {
		stop()
		cancel(nil)
	}
	stopCleanup := context.AfterFunc(ctx, func() { stop() })
	resp, err := base.RoundTrip(req.WithContext(ctx))
	finish := func() {
		stopCleanup()
		cleanup()
	}
	if err != nil || resp == nil || resp.Body == nil || resp.Body == http.NoBody {
		finish()
	} else {
		resp.Body = &sessionBody{ReadCloser: resp.Body, finish: finish}
	}
	return resp, err
}

func isLoopback(host string) bool {
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}

// sessionBody 将撤销监听保留到响应读取完成或关闭。
type sessionBody struct {
	io.ReadCloser
	finish func()
}

func (b *sessionBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.finish()
	}
	return n, err
}

func (b *sessionBody) Close() error {
	defer b.finish()
	return b.ReadCloser.Close()
}

// NewCredentialService 使用指定目录与 Vault 初始化凭证管理器。
func NewCredentialService(storageDir string, v vault.Vault) (*CredentialService, error) {
	if storageDir == "" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			storageDir = filepath.Join(appData, "media-fetcher", "credentials")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("auth: 无法获取用户主目录: %w", err)
			}
			storageDir = filepath.Join(home, ".media-fetcher", "credentials")
		}
	}

	if err := os.MkdirAll(storageDir, 0700); err != nil {
		return nil, fmt.Errorf("auth: 创建存储目录失败: %w", err)
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
		// 忽略初始无元数据文件的情况
	}

	// 恢复已加载账号的凭证 Jar，并开启运行时自动持久化
	for key, acc := range s.accounts {
		credFile := s.credFilePath(acc.SiteID, acc.AccountID)
		jar := cookiejar.New(acc.SiteID, acc.AccountID)
		if err := jar.Load(credFile, s.vault); err != nil {
			acc.Status = StatusExpired
			continue
		}
		s.bindPersistence(jar, credFile)
		s.jars[key] = jar
	}

	return s, nil
}

func (s *CredentialService) bindPersistence(jar *cookiejar.PersistentCookieJar, path string) {
	jar.SetPersistErrHandler(func(error) {
		// CookieJar 接口无法返回错误，不能把磁盘路径或凭证写入日志。
		log.Print("auth: 自动保存凭证失败，下次 Cookie 更新时将重试")
	})
	jar.SetAutoPersist(path, s.vault)
}

func accountKey(siteID, accountID string) string {
	return strings.ToLower(siteID) + ":" + accountID
}

func (s *CredentialService) credFilePath(siteID, accountID string) string {
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
		if a != nil {
			s.accounts[accountKey(a.SiteID, a.AccountID)] = a
		}
	}
	return nil
}

func (s *CredentialService) saveAccountsMetaLocked() error {
	var list []*Account
	for _, a := range s.accounts {
		if a != nil {
			list = append(list, a)
		}
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.metaFilePath() + fmt.Sprintf(".tmp.%d", time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return fsutil.PublishFile(tmp, s.metaFilePath(), fsutil.StrategyReplace)
}

// RegisterOrUpdateAccount 登记或更新账号，持久化加密凭证并启用自动落盘。
func (s *CredentialService) RegisterOrUpdateAccount(acc *Account, jar *cookiejar.PersistentCookieJar) error {
	if acc == nil || acc.SiteID == "" || acc.AccountID == "" {
		return errors.New("auth: 无效的账号实体")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := accountKey(acc.SiteID, acc.AccountID)

	// 若为该站点的首个账号，自动置为活动账号
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

	// 持久化加密 Jar 并绑定自动落盘
	if jar != nil {
		if !strings.EqualFold(jar.SiteID(), acc.SiteID) || jar.AccountID() != acc.AccountID {
			return cookiejar.ErrAccountMismatch
		}
		if old := s.jars[key]; old != nil && old != jar {
			old.Revoke()
			delete(s.jars, key)
			s.accounts[key].Status = StatusExpired
		}
		credPath := s.credFilePath(acc.SiteID, acc.AccountID)
		if err := jar.Save(credPath, s.vault); err != nil {
			return fmt.Errorf("auth: 持久化凭证失败: %w", err)
		}
		s.bindPersistence(jar, credPath)
		s.jars[key] = jar
	}

	s.accounts[key] = acc
	return s.saveAccountsMetaLocked()
}

// GetAccount 根据 SiteID 与 AccountID 查询账号信息。
func (s *CredentialService) GetAccount(siteID, accountID string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acc, ok := s.accounts[accountKey(siteID, accountID)]
	if !ok {
		return nil, ErrAccountNotFound
	}
	return acc, nil
}

// GetActiveAccount 返回指定站点的当前活动账号。
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

// SetActiveAccount 切换指定站点的活动账号。
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

// ListAccounts 返回用于 UI 展示的脱敏账号列表。
func (s *CredentialService) ListAccounts(siteID string) []SafeAccount {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []SafeAccount
	for _, acc := range s.accounts {
		if acc != nil && (siteID == "" || strings.EqualFold(acc.SiteID, siteID)) {
			result = append(result, acc.ToSafe())
		}
	}
	return result
}

// GetClient 返回绑定该账号专属 CookieJar 的受控 http.Client。
// 客户端内置重定向安全审查（防止跨站凭据外泄）及会话撤销防御。
func (s *CredentialService) GetClient(siteID, accountID string) (*http.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := accountKey(siteID, accountID)
	jar, ok := s.jars[key]
	if !ok || jar.IsRevoked() || s.accounts[key].Status != StatusActive {
		return nil, fmt.Errorf("%w: %s 账号会话不存在或已撤销", ErrAccountNotFound, key)
	}

	return &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		Transport: &revocableTransport{
			base:   http.DefaultTransport,
			jar:    jar,
			siteID: siteID,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("auth: 重定向次数过多 (超过10次)")
			}
			if !isHostTrustedForSite(siteID, req.URL.Hostname()) {
				// 跨站重定向：立即剥离所有 Cookie 与 Authorization 头部，防止凭据外泄到第三方站点
				req.Header.Del("Cookie")
				req.Header.Del("Authorization")
			}
			return nil
		},
	}, nil
}

// GetAnonymousClient 返回完全隔离的匿名 http.Client。
// 绝对不携带任何已保存凭据，且内置跨站重定向保护。
func (s *CredentialService) GetAnonymousClient(siteID string) *http.Client {
	anonJar := cookiejar.New(siteID, "anonymous")
	return &http.Client{
		Jar:     anonJar,
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("auth: 重定向次数过多 (超过10次)")
			}
			if !isHostTrustedForSite(siteID, req.URL.Hostname()) {
				req.Header.Del("Cookie")
				req.Header.Del("Authorization")
			}
			return nil
		},
	}
}

// DeleteAccount 彻底注销账号：撤销内存 CookieJar、物理移除磁盘加密文件，并更新元数据。
// 若磁盘删除失败且文件仍存在，显式返回错误，禁止掩盖失败。
func (s *CredentialService) DeleteAccount(siteID, accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := accountKey(siteID, accountID)
	acc, ok := s.accounts[key]
	if !ok {
		return ErrAccountNotFound
	}

	// 1. 彻底撤销内存 CookieJar，拒绝后续一切读写
	if jar, exists := s.jars[key]; exists {
		jar.Revoke()
		delete(s.jars, key)
	}

	// 2. 物理删除加密凭证文件
	acc.Status = StatusExpired
	credPath := s.credFilePath(acc.SiteID, acc.AccountID)
	if err := os.Remove(credPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("auth: 物理删除磁盘凭证文件失败: %w", err)
	}

	// 3. 更新活动账号及元数据
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

// ValidateAccount 运行外部验证函数对指定账号的会话进行在线核验。
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
