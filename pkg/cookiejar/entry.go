package cookiejar

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CookieEntry 封装带有完整 RFC 6265 元数据的 Cookie，用于持久化存储与精准匹配。
type CookieEntry struct {
	Name           string    `json:"name"`
	Value          string    `json:"value"`
	Domain         string    `json:"domain"`
	Path           string    `json:"path"`
	HostOnly       bool      `json:"host_only"`
	Secure         bool      `json:"secure"`
	HttpOnly       bool      `json:"http_only"`
	SameSite       string    `json:"same_site,omitempty"`
	CreationTime   time.Time `json:"creation_time"`
	LastAccessTime time.Time `json:"last_access_time"`
	Expires        time.Time `json:"expires,omitempty"`
	HasExpires     bool      `json:"has_expires"`
	MaxAge         int       `json:"max_age,omitempty"`
}

// IsExpired 判断 Cookie 在指定时间点 at 是否已过期。
func (e *CookieEntry) IsExpired(at time.Time) bool {
	if !e.HasExpires {
		return false
	}
	return at.After(e.Expires) || at.Equal(e.Expires)
}

// Key 返回 RFC 6265 定义的 Cookie 唯一标识符：(Name, Domain, Path)。
// 注意：依据 RFC 6265，Cookie 名称区分大小写，Domain 不区分大小写。
func (e *CookieEntry) Key() string {
	return e.Name + "#" + strings.ToLower(e.Domain) + "#" + e.Path
}

// ToHTTPCookie 将内部实体转换为 Go 标准库 *http.Cookie。
func (e *CookieEntry) ToHTTPCookie() *http.Cookie {
	c := &http.Cookie{
		Name:     e.Name,
		Value:    e.Value,
		Path:     e.Path,
		Domain:   e.Domain,
		Expires:  e.Expires,
		Secure:   e.Secure,
		HttpOnly: e.HttpOnly,
		MaxAge:   e.MaxAge,
	}
	switch strings.ToLower(e.SameSite) {
	case "lax":
		c.SameSite = http.SameSiteLaxMode
	case "strict":
		c.SameSite = http.SameSiteStrictMode
	case "none":
		c.SameSite = http.SameSiteNoneMode
	default:
		c.SameSite = http.SameSiteDefaultMode
	}
	return c
}

// MatchURL 检查此 Cookie 实体在当前时刻 now 是否应当发送给目标 URL。
func (e *CookieEntry) MatchURL(u *url.URL, now time.Time) bool {
	if e.IsExpired(now) {
		return false
	}

	// 1. 安全标记检查 (Secure Flag)
	if e.Secure {
		scheme := strings.ToLower(u.Scheme)
		if scheme != "https" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" {
			return false
		}
	}

	// 2. 域名匹配检查 (Domain Matching)
	host := strings.ToLower(u.Hostname())
	if !domainMatch(host, strings.ToLower(e.Domain), e.HostOnly) {
		return false
	}

	// 3. 路径匹配检查 (Path Matching)
	reqPath := u.Path
	if reqPath == "" {
		reqPath = "/"
	}
	if !pathMatch(reqPath, e.Path) {
		return false
	}

	return true
}

// domainMatch 实现 RFC 6265 5.1.3 节的域名匹配算法。
func domainMatch(host, domain string, hostOnly bool) bool {
	if host == domain {
		return true
	}
	if hostOnly {
		return false
	}
	// IP 地址不能作为子域匹配
	if net.ParseIP(host) != nil {
		return false
	}
	// 必须以 "." + domain 结尾
	if strings.HasSuffix(host, "."+domain) {
		return true
	}
	return false
}

// pathMatch 实现 RFC 6265 5.1.4 节的路径匹配算法。
func pathMatch(requestPath, cookiePath string) bool {
	if requestPath == cookiePath {
		return true
	}
	if strings.HasPrefix(requestPath, cookiePath) {
		if strings.HasSuffix(cookiePath, "/") {
			return true
		}
		if len(requestPath) > len(cookiePath) && requestPath[len(cookiePath)] == '/' {
			return true
		}
	}
	return false
}

// defaultPath 依据 RFC 6265 5.1.4 节计算默认路径。
func defaultPath(uriPath string) string {
	if uriPath == "" || uriPath[0] != '/' {
		return "/"
	}
	lastSlash := strings.LastIndex(uriPath, "/")
	if lastSlash <= 0 {
		return "/"
	}
	return uriPath[:lastSlash]
}
