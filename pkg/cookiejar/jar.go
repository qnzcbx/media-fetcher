package cookiejar

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"media-fetcher/pkg/vault"
)

var (
	// ErrInvalidSchemaVersion 表示不支持的持久化版本。
	ErrInvalidSchemaVersion = errors.New("cookiejar: 不支持的凭证 Schema 版本")
	// ErrAccountMismatch 表示凭证文件中的 SiteID 或 AccountID 与当前 CookieJar 不匹配。
	ErrAccountMismatch = errors.New("cookiejar: site_id 或 account_id 不匹配")
	// ErrJarRevoked 表示该 Cookie 会话已被注销撤销，拒绝一切读写。
	ErrJarRevoked = errors.New("cookiejar: 会话已被撤销注销")
)

// defaultTrustedDomains 定义各站点的默认信任根域名白名单，防范跨站窃取与污染。
var defaultTrustedDomains = map[string][]string{
	"bilibili": {"bilibili.com", "hdslb.com", "bilivideo.com", "bilivideo.cn", "bilibili.tv"},
	"lofter":   {"lofter.com", "126.net", "163.com"},
}

// PersistentCookieJar 完整实现 http.CookieJar 接口，具备 RFC 6265 精确状态追踪、
// 公共后缀防御、站点信任边界拦截、版本化加密持久化、运行时自动落盘及主动撤销机制。
type PersistentCookieJar struct {
	mu             sync.RWMutex
	persistMu      sync.Mutex
	siteID         string
	accountID      string
	entries        map[string]CookieEntry // 键为 CookieEntry.Key()
	allowedDomains []string               // 站点受信任域名白名单
	persistPath    string                 // 运行时自动持久化路径
	vault          vault.Vault            // 运行时自动持久化 Vault
	isRevoked      bool                   // 会话撤销标记（注销后拒绝读写）
}

// New 创建针对特定 SiteID 与 AccountID 的 PersistentCookieJar 实例。
func New(siteID, accountID string) *PersistentCookieJar {
	jar := &PersistentCookieJar{
		siteID:    siteID,
		accountID: accountID,
		entries:   make(map[string]CookieEntry),
	}
	// 加载默认站点信任域
	if defaults, ok := defaultTrustedDomains[strings.ToLower(siteID)]; ok {
		jar.allowedDomains = append(jar.allowedDomains, defaults...)
	}
	return jar
}

// SetAllowedDomains 显式设置或扩展该 CookieJar 允许的受信任域名白名单（用于测试或扩展站点）。
func (j *PersistentCookieJar) SetAllowedDomains(domains []string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.allowedDomains = make([]string, len(domains))
	copy(j.allowedDomains, domains)
}

// SetAutoPersist 配置运行时自动持久化路径与加密 Vault。
// 当 SetCookies 发生新增、变更或删除时，会自动同步加密落盘，防止重启状态回退。
func (j *PersistentCookieJar) SetAutoPersist(filePath string, v vault.Vault) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.persistPath = filePath
	j.vault = v
}

// SiteID 返回站点标识。
func (j *PersistentCookieJar) SiteID() string {
	return j.siteID
}

// AccountID 返回账号标识。
func (j *PersistentCookieJar) AccountID() string {
	return j.accountID
}

// Revoke 彻底撤销并废弃当前会话。
// 标记撤销后，内存条目被立即清空，后续任何 SetCookies 写入或 Cookies 查询均被直接拒绝。
func (j *PersistentCookieJar) Revoke() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.isRevoked = true
	j.entries = make(map[string]CookieEntry)
	j.persistPath = ""
	j.vault = nil
}

// IsRevoked 检查会话是否已被撤销。
func (j *PersistentCookieJar) IsRevoked() bool {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.isRevoked
}

// isHostTrustedLocked 校验主机名是否处于当前 SiteID 的信任边界内。
func (j *PersistentCookieJar) isHostTrustedLocked(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	// 回环地址始终允许（用于本地单元测试与 Mock 服务器）
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	// 如果当前站点未配置受信任域名白名单，则放行
	if len(j.allowedDomains) == 0 {
		return true
	}
	for _, allowed := range j.allowedDomains {
		allowed = strings.ToLower(allowed)
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

// SetCookies 处理来自 HTTP 响应的 Set-Cookie 头部，严格执行站点信任、公共后缀及删除逻辑。
func (j *PersistentCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if u == nil || len(cookies) == 0 {
		return
	}

	j.mu.Lock()
	if j.isRevoked {
		j.mu.Unlock()
		return
	}

	host := strings.ToLower(u.Hostname())
	// 站点信任边界检查：响应来源 Host 必须属于当前 siteID 的信任域
	if !j.isHostTrustedLocked(host) {
		j.mu.Unlock()
		return
	}

	now := time.Now()
	hasMutated := false

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
			// 去除前导点 (RFC 6265)
			domain = strings.TrimPrefix(domain, ".")

			// 安全检查 1：公共后缀拒绝 (Public Suffix Check)
			if IsPublicSuffix(domain) && domain != host {
				// 严格拒绝将 Cookie 扩大至整个顶级域或公共二级域（如 .com / .com.cn）
				continue
			}

			// 安全检查 2：域匹配检查，响应来源 Host 必须域匹配 Cookie 指定的 Domain
			if !domainMatch(host, domain, false) {
				continue
			}

			// 安全检查 3：Cookie Domain 也必须在站点受信任白名单内
			if !j.isHostTrustedLocked(domain) {
				continue
			}
		}

		// 确定 Cookie 路径
		cookiePath := c.Path
		if cookiePath == "" || cookiePath[0] != '/' {
			cookiePath = defaultPath(u.Path)
		}

		// 处理生命周期与过期
		hasExpires := false
		var expires time.Time
		isDelete := false

		if c.MaxAge < 0 {
			isDelete = true
		} else if c.MaxAge > 0 {
			hasExpires = true
			// 防止正 MaxAge 过大导致乘法溢出，上限保护为 20 年
			maxAgeSec := c.MaxAge
			if maxAgeSec > 630720000 {
				maxAgeSec = 630720000
			}
			expires = now.Add(time.Duration(maxAgeSec) * time.Second)
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
			if _, exists := j.entries[key]; exists {
				delete(j.entries, key)
				hasMutated = true
			}
		} else {
			// RFC 6265: 覆盖同键 Cookie 时保留最初的 CreationTime
			if old, exists := j.entries[key]; exists {
				entry.CreationTime = old.CreationTime
			}
			j.entries[key] = entry
			hasMutated = true
		}
	}

	// 如果开启了运行时自动持久化且发生了变更，触发安全落盘
	persistPath := j.persistPath
	vaultInstance := j.vault
	j.mu.Unlock()

	if hasMutated && persistPath != "" && vaultInstance != nil {
		_ = j.Save(persistPath, vaultInstance)
	}
}

// Cookies 返回匹配目标 URL 且未过期的 Cookie 列表。
// 依据 RFC 6265 5.4 节要求：路径较长的 Cookie 排在前面；路径长度相同时，创建时间较早的排在前面。
func (j *PersistentCookieJar) Cookies(u *url.URL) []*http.Cookie {
	if u == nil {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	if j.isRevoked {
		return nil
	}

	host := strings.ToLower(u.Hostname())
	// 站点信任边界检查：请求目标 Host 必须属于当前 siteID 的信任域
	if !j.isHostTrustedLocked(host) {
		return nil
	}

	now := time.Now()
	var matched []CookieEntry
	var expiredKeys []string

	for key, entry := range j.entries {
		if entry.IsExpired(now) {
			expiredKeys = append(expiredKeys, key)
			continue
		}
		if entry.MatchURL(u, now) {
			entry.LastAccessTime = now
			j.entries[key] = entry
			matched = append(matched, entry)
		}
	}

	// 清理已过期的 Cookie
	for _, k := range expiredKeys {
		delete(j.entries, k)
	}

	// RFC 6265 5.4 排序：最长路径优先，同长度下最先创建优先
	sort.Slice(matched, func(i, j int) bool {
		if len(matched[i].Path) != len(matched[j].Path) {
			return len(matched[i].Path) > len(matched[j].Path)
		}
		return matched[i].CreationTime.Before(matched[j].CreationTime)
	})

	var result []*http.Cookie
	for _, m := range matched {
		result = append(result, m.ToHTTPCookie())
	}

	return result
}

// AllEntries 返回所有处于活跃、未过期状态的 Cookie 实体切片快照。
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

// Clear 清空内存中当前存储的全部 Cookie。
func (j *PersistentCookieJar) Clear() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.entries = make(map[string]CookieEntry)
}

// Save 将活跃 Cookie 序列化为版本化 JSON，经由 Vault 加密后原子落地到磁盘文件。
func (j *PersistentCookieJar) Save(filePath string, v vault.Vault) error {
	if v == nil {
		return errors.New("cookiejar: vault 不能为空")
	}

	j.persistMu.Lock()
	defer j.persistMu.Unlock()

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
		return fmt.Errorf("cookiejar: 序列化 schema 失败: %w", err)
	}

	ciphertext, err := v.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("cookiejar: 凭证加密失败: %w", err)
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cookiejar: 创建存储目录失败: %w", err)
	}

	// 独占原子写入：写入独立临时文件后原子重命名
	tmpPath := filePath + fmt.Sprintf(".tmp.%d.%d", os.Getpid(), time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, ciphertext, 0600); err != nil {
		return fmt.Errorf("cookiejar: 写入临时文件失败: %w", err)
	}

	if err := atomicReplaceFile(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("cookiejar: 原子替换凭证文件失败: %w", err)
	}

	return nil
}

// Load 读取磁盘加密凭证，使用 Vault 解密，验证版本号并恢复活跃 Cookie。
func (j *PersistentCookieJar) Load(filePath string, v vault.Vault) error {
	if v == nil {
		return errors.New("cookiejar: vault 不能为空")
	}

	j.persistMu.Lock()
	defer j.persistMu.Unlock()

	ciphertext, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cookiejar: 读取凭证文件失败: %w", err)
	}

	plaintext, err := v.Decrypt(ciphertext)
	if err != nil {
		return fmt.Errorf("cookiejar: 解密凭证失败: %w", err)
	}

	var schema StorageSchema
	if err := json.Unmarshal(plaintext, &schema); err != nil {
		return fmt.Errorf("cookiejar: 解析凭证 JSON 失败: %w", err)
	}

	if schema.Version != CurrentSchemaVersion {
		return fmt.Errorf("%w: 存储版本 %d，支持版本 %d", ErrInvalidSchemaVersion, schema.Version, CurrentSchemaVersion)
	}

	if schema.SiteID != j.siteID || schema.AccountID != j.accountID {
		return fmt.Errorf("%w: 文件记录 (%s/%s) 与当前 Jar (%s/%s)",
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
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	_ = os.Remove(dst)
	return os.Rename(src, dst)
}
