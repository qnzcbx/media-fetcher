package auth

import (
	"net/http"
	"net/url"
	"os"
	"strings"
)

// newBaseTransport 构建与外部工具代理优先级一致的基础 HTTP Transport：
// 1. 优先尊重环境变量代理（HTTP_PROXY / HTTPS_PROXY / NO_PROXY）；
// 2. Windows 下未设置环境变量代理时，自动回退读取系统代理（Internet Settings，只读）；
// 3. 均未启用时直连（适配 TUN 虚拟网卡模式）。
func newBaseTransport() *http.Transport {
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = proxyFromEnvOrSystem
	if base.MaxIdleConnsPerHost < 8 {
		base.MaxIdleConnsPerHost = 8
	}
	return base
}

func getEnvAny(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func envProxyForRequest(req *http.Request) (*url.URL, bool, error) {
	if req == nil || req.URL == nil {
		return nil, false, nil
	}
	scheme := strings.ToLower(req.URL.Scheme)
	var raw string
	switch scheme {
	case "https":
		raw = getEnvAny("HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy")
	default:
		raw = getEnvAny("HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy")
	}
	if raw == "" {
		// 检查是否设置了任一代理环境变量；若完全未设则返回 configured=false 以便回退系统代理
		if getEnvAny("HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy") == "" {
			return nil, false, nil
		}
		return nil, true, nil
	}
	if noProxy := getEnvAny("NO_PROXY", "no_proxy"); noProxy != "" && shouldBypassNoProxy(req.URL.Hostname(), noProxy) {
		return nil, true, nil
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, true, err
	}
	return u, true, nil
}

func shouldBypassNoProxy(host, rawNoProxy string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return true
	}
	for _, part := range strings.Split(rawNoProxy, ",") {
		p := strings.ToLower(strings.TrimSpace(part))
		if p == "" {
			continue
		}
		if p == "*" {
			return true
		}
		if idx := strings.Index(p, ":"); idx >= 0 && !strings.HasPrefix(p, "[") {
			p = p[:idx]
		}
		if strings.HasPrefix(p, ".") {
			if strings.HasSuffix(h, p) || h == strings.TrimPrefix(p, ".") {
				return true
			}
			continue
		}
		if h == p || strings.HasSuffix(h, "."+p) {
			return true
		}
	}
	return false
}

func proxyFromEnvOrSystem(req *http.Request) (*url.URL, error) {
	if u, configured, err := envProxyForRequest(req); configured {
		return u, err
	}
	return systemProxyForRequest(req)
}

// parseSystemProxyServer 解析 Windows ProxyServer 配置（支持单地址 "host:port" 与分协议 "http=...;https=...;socks=..."）。
func parseSystemProxyServer(rawServer, reqScheme string) (*url.URL, error) {
	server := strings.TrimSpace(rawServer)
	if server == "" {
		return nil, nil
	}
	scheme := strings.ToLower(strings.TrimSpace(reqScheme))
	if scheme == "" {
		scheme = "http"
	}

	var chosen string
	if strings.Contains(server, "=") {
		entries := map[string]string{}
		for _, part := range strings.Split(server, ";") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			k, v, ok := strings.Cut(part, "=")
			if !ok {
				continue
			}
			k = strings.ToLower(strings.TrimSpace(k))
			v = strings.TrimSpace(v)
			if k != "" && v != "" {
				entries[k] = v
			}
		}
		if v, ok := entries[scheme]; ok {
			chosen = v
		} else if scheme == "https" && entries["http"] != "" {
			chosen = entries["http"]
		} else if entries["socks"] != "" {
			chosen = entries["socks"]
			if !strings.Contains(chosen, "://") {
				chosen = "socks5://" + chosen
			}
		}
	} else {
		chosen = server
	}

	if chosen == "" {
		return nil, nil
	}
	if !strings.Contains(chosen, "://") {
		chosen = "http://" + chosen
	}
	u, err := url.Parse(chosen)
	if err != nil || u.Host == "" {
		return nil, nil
	}
	return u, nil
}

// shouldBypassSystemProxy 按 Windows ProxyOverride 规则判断目标主机是否应直连绕过系统代理。
func shouldBypassSystemProxy(host, rawOverride string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" || isLoopback(h) {
		return true
	}
	override := strings.TrimSpace(rawOverride)
	if override == "" {
		return false
	}
	for _, token := range strings.Split(override, ";") {
		pat := strings.ToLower(strings.TrimSpace(token))
		if pat == "" {
			continue
		}
		if pat == "<local>" {
			if !strings.Contains(h, ".") {
				return true
			}
			continue
		}
		// 去除可选协议前缀或端口
		if idx := strings.Index(pat, "://"); idx >= 0 {
			pat = pat[idx+3:]
		}
		if idx := strings.Index(pat, ":"); idx >= 0 && !strings.HasPrefix(pat, "[") {
			pat = pat[:idx]
		}
		if wildcardMatch(pat, h) {
			return true
		}
	}
	return false
}

func wildcardMatch(pattern, target string) bool {
	if pattern == "*" || pattern == target {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return false
	}
	parts := strings.Split(pattern, "*")
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			if !strings.HasPrefix(target, part) {
				return false
			}
			pos = len(part)
			continue
		}
		if i == len(parts)-1 {
			return strings.HasSuffix(target[pos:], part)
		}
		idx := strings.Index(target[pos:], part)
		if idx < 0 {
			return false
		}
		pos += idx + len(part)
	}
	return true
}
