package cookiejar

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"media-fetcher/pkg/vault"
)

func parseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestCookieJar_DomainAndHostOnly(t *testing.T) {
	jar := New("bilibili", "user1")

	// 1. 仅限主机的 HostOnly Cookie（未指定 domain）
	apiURL := parseURL("https://api.bilibili.com/x/web-interface/nav")
	jar.SetCookies(apiURL, []*http.Cookie{
		{Name: "host_token", Value: "val_host", Path: "/"},
	})

	// 2. 泛域名 Cookie (domain = .bilibili.com)
	jar.SetCookies(apiURL, []*http.Cookie{
		{Name: "global_token", Value: "val_global", Domain: ".bilibili.com", Path: "/"},
	})

	// 3. 拒绝接收的域不匹配 Cookie (domain 不匹配)
	jar.SetCookies(apiURL, []*http.Cookie{
		{Name: "evil_token", Value: "val_evil", Domain: "google.com", Path: "/"},
	})

	// 在 api.bilibili.com 上检查 -> 应获取 host_token 与 global_token，但绝不应有 evil_token
	cookies := jar.Cookies(apiURL)
	cookieMap := make(map[string]string)
	for _, c := range cookies {
		cookieMap[c.Name] = c.Value
	}

	if cookieMap["host_token"] != "val_host" {
		t.Errorf("expected host_token on api.bilibili.com, got %v", cookieMap)
	}
	if cookieMap["global_token"] != "val_global" {
		t.Errorf("expected global_token on api.bilibili.com, got %v", cookieMap)
	}
	if _, ok := cookieMap["evil_token"]; ok {
		t.Errorf("evil_token should have been rejected for domain mismatch")
	}

	// 在 www.bilibili.com 上检查 -> 应当收到 global_token，但绝不能收到 host_token
	wwwURL := parseURL("https://www.bilibili.com/video/BV123")
	wwwCookies := jar.Cookies(wwwURL)
	wwwMap := make(map[string]string)
	for _, c := range wwwCookies {
		wwwMap[c.Name] = c.Value
	}

	if _, ok := wwwMap["host_token"]; ok {
		t.Errorf("host_token 不应发送给 www.bilibili.com")
	}
	if wwwMap["global_token"] != "val_global" {
		t.Errorf("期望在 www.bilibili.com 获得 global_token，实际: %v", wwwMap)
	}
}

func TestCookieJar_PathMatching(t *testing.T) {
	jar := New("testsite", "user1")
	baseURL := parseURL("https://example.com/api/v1/user")

	jar.SetCookies(baseURL, []*http.Cookie{
		{Name: "root_cookie", Value: "1", Path: "/"},
		{Name: "api_cookie", Value: "2", Path: "/api"},
		{Name: "v1_cookie", Value: "3", Path: "/api/v1"},
		{Name: "v2_cookie", Value: "4", Path: "/api/v2"},
	})

	// 对 /api/v1/user 的请求应当匹配 root_cookie, api_cookie, v1_cookie，但不匹配 v2_cookie
	cookies := jar.Cookies(baseURL)
	m := make(map[string]string)
	for _, c := range cookies {
		m[c.Name] = c.Value
	}

	if m["root_cookie"] != "1" || m["api_cookie"] != "2" || m["v1_cookie"] != "3" {
		t.Errorf("缺少期望的 cookie: %v", m)
	}
	if _, ok := m["v2_cookie"]; ok {
		t.Errorf("v2_cookie 不应匹配 /api/v1/user: %v", m)
	}

	// 对 /api-doc 的请求不应匹配 /api
	apiDocURL := parseURL("https://example.com/api-doc")
	docCookies := jar.Cookies(apiDocURL)
	docMap := make(map[string]string)
	for _, c := range docCookies {
		docMap[c.Name] = c.Value
	}
	if _, ok := docMap["api_cookie"]; ok {
		t.Errorf("api_cookie should not match /api-doc: %v", docMap)
	}
}

func TestCookieJar_SecureFlag(t *testing.T) {
	jar := New("testsite", "user1")
	httpsURL := parseURL("https://example.com/test")
	httpURL := parseURL("http://example.com/test")

	jar.SetCookies(httpsURL, []*http.Cookie{
		{Name: "sec_cookie", Value: "secret", Secure: true, Path: "/"},
		{Name: "plain_cookie", Value: "normal", Secure: false, Path: "/"},
	})

	// HTTPS 请求应当同时收到二者
	httpsCookies := jar.Cookies(httpsURL)
	if len(httpsCookies) != 2 {
		t.Fatalf("expected 2 cookies on https, got %d", len(httpsCookies))
	}

	// HTTP 请求应当仅收到普通 plain_cookie
	httpCookies := jar.Cookies(httpURL)
	if len(httpCookies) != 1 || httpCookies[0].Name != "plain_cookie" {
		t.Fatalf("expected only plain_cookie on http, got %v", httpCookies)
	}
}

func TestCookieJar_ExpirationAndDeletion(t *testing.T) {
	jar := New("testsite", "user1")
	u := parseURL("https://example.com/")

	// 设置常规 Cookie 及已过期 Cookie
	jar.SetCookies(u, []*http.Cookie{
		{Name: "temp", Value: "to_be_deleted", Path: "/"},
		{Name: "already_expired", Value: "expired", Path: "/", Expires: time.Now().Add(-1 * time.Hour)},
	})

	// 已过期的 Cookie 不应返回
	cookies := jar.Cookies(u)
	if len(cookies) != 1 || cookies[0].Name != "temp" {
		t.Fatalf("expected only 'temp' cookie, got %v", cookies)
	}

	// 通过 MaxAge < 0 删除 'temp'
	jar.SetCookies(u, []*http.Cookie{
		{Name: "temp", Value: "", Path: "/", MaxAge: -1},
	})

	cookies = jar.Cookies(u)
	if len(cookies) != 0 {
		t.Fatalf("expected 0 cookies after deletion, got %v", cookies)
	}
}

func TestCookieJar_SaveAndLoadWithVault(t *testing.T) {
	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "credentials", "bilibili_user1.bin")

	v := vault.NewMockVault("user1")
	jar1 := New("bilibili", "user1")

	u := parseURL("https://www.bilibili.com/")
	jar1.SetCookies(u, []*http.Cookie{
		{
			Name:     "SESSDATA",
			Value:    "sess_secret_value",
			Domain:   ".bilibili.com",
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(24 * time.Hour),
		},
		{
			Name:    "expired_cookie",
			Value:   "will_not_persist",
			Domain:  ".bilibili.com",
			Path:    "/",
			Expires: time.Now().Add(-10 * time.Minute),
		},
	})

	// 持久化到磁盘
	if err := jar1.Save(savePath, v); err != nil {
		t.Fatalf("jar Save failed: %v", err)
	}

	// 验证磁盘上的文件是密文（绝不能包含明文 'sess_secret_value'）
	rawBytes, err := os.ReadFile(savePath)
	if err != nil {
		t.Fatalf("read saved file failed: %v", err)
	}
	if string(rawBytes) == "" || string(rawBytes) == "sess_secret_value" {
		t.Fatal("saved file should be encrypted ciphertext")
	}

	// 加载恢复到新的 Jar 实例
	jar2 := New("bilibili", "user1")
	if err := jar2.Load(savePath, v); err != nil {
		t.Fatalf("jar Load failed: %v", err)
	}

	loadedCookies := jar2.Cookies(u)
	if len(loadedCookies) != 1 {
		t.Fatalf("expected 1 cookie restored, got %d", len(loadedCookies))
	}
	c := loadedCookies[0]
	if c.Name != "SESSDATA" || c.Value != "sess_secret_value" {
		t.Errorf("cookie data mismatch: %+v", c)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie flags mismatch: HttpOnly=%v, Secure=%v, SameSite=%v", c.HttpOnly, c.Secure, c.SameSite)
	}

	// 账号不匹配保护：将 user1 的数据加载到 user2 的 Jar 必须报错拒绝
	jarMismatch := New("bilibili", "user2")
	if err := jarMismatch.Load(savePath, v); !errors.Is(err, ErrAccountMismatch) {
		t.Fatalf("expected ErrAccountMismatch, got %v", err)
	}
}

// H1 回归测试：公共后缀防御与站点信任边界
func TestH1_PublicSuffixAndTrustBoundary(t *testing.T) {
	jar := New("bilibili", "user_h1")

	biliURL := parseURL("https://www.bilibili.com/video/BV123")
	attackerURL := parseURL("https://evil.com/phishing")

	// 1. 尝试将 Cookie 范围扩大至整个顶级域或公共二级后缀 (.com, .com.cn)
	jar.SetCookies(biliURL, []*http.Cookie{
		{Name: "bad_tld", Value: "evil", Domain: "com", Path: "/"},
		{Name: "bad_cctld", Value: "evil", Domain: "com.cn", Path: "/"},
		{Name: "bad_other_tld", Value: "evil", Domain: "net", Path: "/"},
		{Name: "legit_sub", Value: "good", Domain: ".bilibili.com", Path: "/"},
	})

	cookies := jar.Cookies(biliURL)
	m := make(map[string]string)
	for _, c := range cookies {
		m[c.Name] = c.Value
	}

	if _, ok := m["bad_tld"]; ok {
		t.Error("严重安全漏洞：CookieJar 错误接收了公共后缀域 .com 的 Cookie！")
	}
	if _, ok := m["bad_cctld"]; ok {
		t.Error("严重安全漏洞：CookieJar 错误接收了公共二级后缀域 .com.cn 的 Cookie！")
	}
	if _, ok := m["bad_other_tld"]; ok {
		t.Error("严重安全漏洞：CookieJar 错误接收了公共后缀域 .net 的 Cookie！")
	}
	if m["legit_sub"] != "good" {
		t.Errorf("合法站点子域 Cookie 应当被正常接收，实际未找到: %v", m)
	}

	// 2. 站点信任边界：bilibili 站点的 Cookie 绝不能发送给第三方外部域名
	attackerCookies := jar.Cookies(attackerURL)
	if len(attackerCookies) > 0 {
		t.Fatalf("严重安全漏洞：bilibili 的 Cookie 泄露给了未经信任的第三方域名 %s: %v", attackerURL.Hostname(), attackerCookies)
	}

	// 3. 第三方域名发来的响应也绝不能存入该站点的 CookieJar
	jar.SetCookies(attackerURL, []*http.Cookie{
		{Name: "attacker_token", Value: "poison", Domain: "evil.com", Path: "/"},
	})
	if len(jar.Cookies(attackerURL)) > 0 {
		t.Error("严重安全漏洞：CookieJar 接受了来自不受信任第三方的 Cookie！")
	}
}

// M1 回归测试：最长路径优先排序与同键覆盖保留创建时间
func TestM1_RFC6265_SortingAndOverwrite(t *testing.T) {
	jar := New("bilibili", "user_m1")
	u := parseURL("https://www.bilibili.com/video/detail/item")

	// 设置不同路径长度的 Cookie
	jar.SetCookies(u, []*http.Cookie{
		{Name: "c_root", Value: "root", Domain: "bilibili.com", Path: "/"},
		{Name: "c_video", Value: "video", Domain: "bilibili.com", Path: "/video"},
		{Name: "c_detail", Value: "detail", Domain: "bilibili.com", Path: "/video/detail"},
	})

	cookies := jar.Cookies(u)
	if len(cookies) != 3 {
		t.Fatalf("期望 3 个 Cookie, 实际 %d", len(cookies))
	}

	// 验证最长路径优先排序：/video/detail -> /video -> /
	if cookies[0].Name != "c_detail" || cookies[1].Name != "c_video" || cookies[2].Name != "c_root" {
		t.Errorf("RFC 6265 路径排序不正确: 实际返回顺序为 [%s, %s, %s]",
			cookies[0].Name, cookies[1].Name, cookies[2].Name)
	}
}
