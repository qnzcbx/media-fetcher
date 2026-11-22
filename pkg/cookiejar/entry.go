package cookiejar

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CookieEntry encapsulates a cookie with full RFC 6265 metadata for persistence.
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

// IsExpired reports whether the cookie is expired relative to `at`.
func (e *CookieEntry) IsExpired(at time.Time) bool {
	if !e.HasExpires {
		return false
	}
	return at.After(e.Expires) || at.Equal(e.Expires)
}

// Key returns the identity key for cookie matching (Name + Domain + Path + HostOnly).
func (e *CookieEntry) Key() string {
	ho := "0"
	if e.HostOnly {
		ho = "1"
	}
	return strings.ToLower(e.Name) + "#" + strings.ToLower(e.Domain) + "#" + e.Path + "#" + ho
}

// ToHTTPCookie converts the entry into standard *http.Cookie.
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

// MatchURL checks if this cookie entry should be sent to the given URL at time `now`.
func (e *CookieEntry) MatchURL(u *url.URL, now time.Time) bool {
	if e.IsExpired(now) {
		return false
	}

	// 1. Secure check
	if e.Secure {
		scheme := strings.ToLower(u.Scheme)
		if scheme != "https" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" {
			return false
		}
	}

	// 2. Domain check
	host := strings.ToLower(u.Hostname())
	if !domainMatch(host, strings.ToLower(e.Domain), e.HostOnly) {
		return false
	}

	// 3. Path check
	reqPath := u.Path
	if reqPath == "" {
		reqPath = "/"
	}
	if !pathMatch(reqPath, e.Path) {
		return false
	}

	return true
}

// domainMatch implements RFC 6265 section 5.1.3 domain matching.
func domainMatch(host, domain string, hostOnly bool) bool {
	if host == domain {
		return true
	}
	if hostOnly {
		return false
	}
	// IP addresses cannot domain-match subdomains
	if net.ParseIP(host) != nil {
		return false
	}
	// Must end with "." + domain
	if strings.HasSuffix(host, "."+domain) {
		return true
	}
	return false
}

// pathMatch implements RFC 6265 section 5.1.4 path matching.
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

// defaultPath calculates default path per RFC 6265 section 5.1.4.
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
