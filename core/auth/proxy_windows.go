//go:build windows

package auth

import (
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"
)

type winProxyConfig struct {
	enabled  bool
	server   string
	override string
	readAt   time.Time
}

var (
	winProxyMu  sync.Mutex
	winProxyCfg winProxyConfig
)

const winProxyCacheTTL = 2 * time.Second

func readWindowsSystemProxy() winProxyConfig {
	winProxyMu.Lock()
	defer winProxyMu.Unlock()

	now := time.Now()
	if !winProxyCfg.readAt.IsZero() && now.Sub(winProxyCfg.readAt) < winProxyCacheTTL {
		return winProxyCfg
	}

	cfg := winProxyConfig{readAt: now}
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err == nil {
		defer key.Close()
		if v, _, err := key.GetIntegerValue("ProxyEnable"); err == nil && v == 1 {
			cfg.enabled = true
			if s, _, err := key.GetStringValue("ProxyServer"); err == nil {
				cfg.server = s
			}
			if o, _, err := key.GetStringValue("ProxyOverride"); err == nil {
				cfg.override = o
			}
		}
	}
	winProxyCfg = cfg
	return cfg
}

func systemProxyForRequest(req *http.Request) (*url.URL, error) {
	if req == nil || req.URL == nil {
		return nil, nil
	}
	cfg := readWindowsSystemProxy()
	if !cfg.enabled || cfg.server == "" {
		return nil, nil
	}
	if shouldBypassSystemProxy(req.URL.Hostname(), cfg.override) {
		return nil, nil
	}
	return parseSystemProxyServer(cfg.server, req.URL.Scheme)
}
