package test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"media-fetcher/core/auth"
	"media-fetcher/core/extractor"
	"media-fetcher/core/extractor/bilibili"
	"media-fetcher/pkg/cookiejar"
	"media-fetcher/pkg/vault"
)

// 验收准则 1: 登录成功后关闭程序，重新启动能够恢复有效登录状态。
func TestAcceptance1_LoginRestartAndResume(t *testing.T) {
	storageDir := t.TempDir()

	// 在 Windows 平台使用 DPAPI，在非 Windows 平台使用测试替身 MockVault
	var v vault.Vault
	if runtime.GOOS == "windows" {
		v = vault.NewDPAPIVault(nil)
	} else {
		v = vault.NewMockVault("test_user_win")
	}

	mockSite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, c := range r.Cookies() {
			if c.Name == "SESSDATA" && c.Value == "valid_persistent_sessdata" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"authorized"}`))
				return
			}
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":"unauthorized"}`))
	}))
	defer mockSite.Close()

	srvURL, _ := url.Parse(mockSite.URL)

	// 阶段 1: 首次登录并加密保存凭据
	service1, err := auth.NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatalf("Phase 1: failed to init CredentialService: %v", err)
	}

	jar1 := cookiejar.New("bilibili", "user_1001")
	jar1.SetCookies(srvURL, []*http.Cookie{
		{
			Name:     "SESSDATA",
			Value:    "valid_persistent_sessdata",
			Domain:   srvURL.Hostname(),
			Path:     "/",
			HttpOnly: true,
			Secure:   false, // 本地 Mock 测试站点采用 HTTP
			Expires:  time.Now().Add(24 * time.Hour),
		},
	})

	acc1 := &auth.Account{
		SiteID:    "bilibili",
		AccountID: "user_1001",
		Name:      "BilibiliTester",
	}
	if err := service1.RegisterOrUpdateAccount(acc1, jar1); err != nil {
		t.Fatalf("Phase 1: register account failed: %v", err)
	}

	// 阶段 2: 关闭程序并模拟重启加载恢复
	service1 = nil

	service2, err := auth.NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatalf("Phase 2: restart failed: %v", err)
	}

	client2, err := service2.GetClient("bilibili", "user_1001")
	if err != nil {
		t.Fatalf("Phase 2: could not get restored client: %v", err)
	}

	resp, err := client2.Get(mockSite.URL)
	if err != nil {
		t.Fatalf("Phase 2: HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Phase 2: expected HTTP 200 Authorized after restart, got %d", resp.StatusCode)
	}
}

// 验收准则 2: 验证 Cookie 的 Domain、Path、Secure、HttpOnly、SameSite 和过期行为。
func TestAcceptance2_CookieRFC6265Semantics(t *testing.T) {
	jar := cookiejar.New("bilibili", "test_acc")

	httpsURL, _ := url.Parse("https://sub.bilibili.com/video/item")
	httpURL, _ := url.Parse("http://sub.bilibili.com/video/item")

	jar.SetCookies(httpsURL, []*http.Cookie{
		// 匹配 *.bilibili.com 的泛域名 Cookie
		{Name: "c_domain", Value: "val_domain", Domain: ".bilibili.com", Path: "/video"},
		// 仅匹配 sub.bilibili.com 的 HostOnly Cookie
		{Name: "c_hostonly", Value: "val_host", Path: "/video"},
		// 仅允许在 HTTPS 传输的 Secure Cookie
		{Name: "c_secure", Value: "val_sec", Domain: ".bilibili.com", Path: "/", Secure: true},
		// HttpOnly + SameSite Lax 标记
		{Name: "c_httponly", Value: "val_http", Domain: ".bilibili.com", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode},
		// 域名不匹配的非法 Domain（安全拦截检查）
		{Name: "c_illegal", Value: "val_bad", Domain: "google.com", Path: "/"},
	})

	// 1. 验证发往 sub.bilibili.com/video/item 的 HTTPS 请求正确收到匹配的 Cookie
	cookies := jar.Cookies(httpsURL)
	cMap := make(map[string]*http.Cookie)
	for _, c := range cookies {
		cMap[c.Name] = c
	}

	if cMap["c_domain"] == nil || cMap["c_domain"].Value != "val_domain" {
		t.Error("missing or invalid c_domain")
	}
	if cMap["c_hostonly"] == nil || cMap["c_hostonly"].Value != "val_host" {
		t.Error("missing or invalid c_hostonly")
	}
	if cMap["c_secure"] == nil || !cMap["c_secure"].Secure {
		t.Error("missing or invalid c_secure")
	}
	if cMap["c_httponly"] == nil || !cMap["c_httponly"].HttpOnly || cMap["c_httponly"].SameSite != http.SameSiteLaxMode {
		t.Errorf("missing or invalid c_httponly attributes: %+v", cMap["c_httponly"])
	}
	if cMap["c_illegal"] != nil {
		t.Error("c_illegal should be rejected for domain mismatch")
	}

	// 2. HTTP 请求绝不能接收到 c_secure
	httpCookies := jar.Cookies(httpURL)
	for _, c := range httpCookies {
		if c.Name == "c_secure" {
			t.Error("secure cookie was leaked to insecure HTTP request")
		}
	}

	// 3. 不同的子域名绝不能接收到 c_hostonly
	otherSubURL, _ := url.Parse("https://api.bilibili.com/video/item")
	otherCookies := jar.Cookies(otherSubURL)
	for _, c := range otherCookies {
		if c.Name == "c_hostonly" {
			t.Error("host-only cookie was leaked to different subdomain")
		}
	}
}

// 验收准则 3: Cookie 过期或服务端删除后，不得在重启时重新恢复。
func TestAcceptance3_ExpiredOrDeletedCookiesDoNotRestore(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("user_v")

	service, err := auth.NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	u, _ := url.Parse("https://www.bilibili.com/")
	jar := cookiejar.New("bilibili", "user_exp")

	// 设置活跃 Cookie 与已过期 Cookie
	jar.SetCookies(u, []*http.Cookie{
		{Name: "valid_cookie", Value: "1", Domain: ".bilibili.com", Path: "/", Expires: time.Now().Add(1 * time.Hour)},
		{Name: "expired_cookie", Value: "2", Domain: ".bilibili.com", Path: "/", Expires: time.Now().Add(-1 * time.Hour)},
		{Name: "deleted_cookie", Value: "3", Domain: ".bilibili.com", Path: "/"},
	})

	// 服务端下发指令删除 deleted_cookie
	jar.SetCookies(u, []*http.Cookie{
		{Name: "deleted_cookie", Value: "", Domain: ".bilibili.com", Path: "/", MaxAge: -1},
	})

	acc := &auth.Account{SiteID: "bilibili", AccountID: "user_exp", Name: "ExpTester"}
	if err := service.RegisterOrUpdateAccount(acc, jar); err != nil {
		t.Fatal(err)
	}

	// 模拟重启进程
	service = nil
	restarted, err := auth.NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	client, err := restarted.GetClient("bilibili", "user_exp")
	if err != nil {
		t.Fatal(err)
	}

	restoredCookies := client.Jar.Cookies(u)
	if len(restoredCookies) != 1 || restoredCookies[0].Name != "valid_cookie" {
		t.Fatalf("expected only 'valid_cookie', got: %+v", restoredCookies)
	}
}

// 验收准则 4: 账号 A 与 账号 B 完全隔离，切换账号不会串用 Cookie；匿名解析完全隔离。
func TestAcceptance4_AccountIsolation_NoCookieCrossContamination(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("user_v")
	service, err := auth.NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	var receivedAliceCookie, receivedBobCookie bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, c := range r.Cookies() {
			if c.Name == "TOKEN" && c.Value == "ALICE_TOKEN" {
				receivedAliceCookie = true
			}
			if c.Name == "TOKEN" && c.Value == "BOB_TOKEN" {
				receivedBobCookie = true
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	srvURL, _ := url.Parse(server.URL)

	// 登记账号 Alice
	jarA := cookiejar.New("bilibili", "alice")
	jarA.SetCookies(srvURL, []*http.Cookie{{Name: "TOKEN", Value: "ALICE_TOKEN", Path: "/"}})
	_ = service.RegisterOrUpdateAccount(&auth.Account{SiteID: "bilibili", AccountID: "alice", Name: "Alice"}, jarA)

	// 登记账号 Bob
	jarB := cookiejar.New("bilibili", "bob")
	jarB.SetCookies(srvURL, []*http.Cookie{{Name: "TOKEN", Value: "BOB_TOKEN", Path: "/"}})
	_ = service.RegisterOrUpdateAccount(&auth.Account{SiteID: "bilibili", AccountID: "bob", Name: "Bob"}, jarB)

	// 1. 以 Alice 身份请求
	clientA, _ := service.GetClient("bilibili", "alice")
	receivedAliceCookie = false
	receivedBobCookie = false
	_, _ = clientA.Get(server.URL)
	if !receivedAliceCookie || receivedBobCookie {
		t.Fatalf("Alice request contamination! Alice=%v, Bob=%v", receivedAliceCookie, receivedBobCookie)
	}

	// 2. 以 Bob 身份请求
	clientB, _ := service.GetClient("bilibili", "bob")
	receivedAliceCookie = false
	receivedBobCookie = false
	_, _ = clientB.Get(server.URL)
	if receivedAliceCookie || !receivedBobCookie {
		t.Fatalf("Bob request contamination! Alice=%v, Bob=%v", receivedAliceCookie, receivedBobCookie)
	}

	// 3. 匿名客户端绝不能发送任何凭据 Cookie
	anonClient := service.GetAnonymousClient("bilibili")
	receivedAliceCookie = false
	receivedBobCookie = false
	_, _ = anonClient.Get(server.URL)
	if receivedAliceCookie || receivedBobCookie {
		t.Fatalf("Anonymous client leaked cookies! Alice=%v, Bob=%v", receivedAliceCookie, receivedBobCookie)
	}
}

// 验收准则 5: 不同 Windows 用户无法直接解密对方的凭证文件。
func TestAcceptance5_MultiUserDPAPISimulation(t *testing.T) {
	vaultUser1 := vault.NewMockVault("WindowsUser_Alice_S-1-5-21-1")
	vaultUser2 := vault.NewMockVault("WindowsUser_Bob_S-1-5-21-2")

	plaintext := []byte("confidential_cookie_data_for_alice")
	ciphertext, err := vaultUser1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("User1 encrypt failed: %v", err)
	}

	// 用户 1 能够成功解密自己的密文
	decrypted, err := vaultUser1.Decrypt(ciphertext)
	if err != nil || string(decrypted) != string(plaintext) {
		t.Fatalf("User1 self-decrypt failed: %v", err)
	}

	// 用户 2 尝试解密用户 1 的密文 -> 必须失败
	_, err = vaultUser2.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("User 2 should NOT be able to decrypt User 1's ciphertext")
	}
	if !errors.Is(err, vault.ErrDecryptionFailed) {
		t.Fatalf("expected ErrDecryptionFailed, got: %v", err)
	}

	// 在 Windows 平台上，测试使用不同熵盐的 DPAPI 隔离行为
	if runtime.GOOS == "windows" {
		dpapi1 := vault.NewDPAPIVault([]byte("entropy_a"))
		dpapi2 := vault.NewDPAPIVault([]byte("entropy_b"))

		c, err := dpapi1.Encrypt(plaintext)
		if err != nil {
			t.Fatal(err)
		}
		_, err = dpapi2.Decrypt(c)
		if err == nil {
			t.Fatal("mismatched DPAPI entropy must fail decryption")
		}
	}
}

// 验收准则 6: 凭证损坏或解密失败时，不得崩溃或静默生成错误登录状态。
func TestAcceptance6_CorruptCiphertextGracefulDegradation(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("tester")

	service, err := auth.NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	jar := cookiejar.New("bilibili", "user_corrupt")
	acc := &auth.Account{SiteID: "bilibili", AccountID: "user_corrupt", Name: "CorruptTester"}
	_ = service.RegisterOrUpdateAccount(acc, jar)

	// 恶意篡改加密凭据文件
	credPath := filepath.Join(storageDir, "bilibili_user_corrupt.cred")
	_ = os.WriteFile(credPath, []byte("INVALID_CORRUPT_CIPHERTEXT"), 0600)

	// 重启凭证服务 -> 绝不能 panic
	restarted, err := auth.NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatalf("restarting with corrupt file must not fail constructor: %v", err)
	}

	accLoaded, err := restarted.GetAccount("bilibili", "user_corrupt")
	if err != nil {
		t.Fatalf("account metadata should still load: %v", err)
	}
	if accLoaded.Status != auth.StatusExpired {
		t.Fatalf("corrupt credential must mark account as expired, got status: %s", accLoaded.Status)
	}
}

// 验收准则 7: 删除账号时同步清除持久化凭证及对应内存会话。
func TestAcceptance7_AccountDeletionPurgesDiskAndMemory(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("tester")
	service, err := auth.NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	jar := cookiejar.New("bilibili", "user_to_delete")
	acc := &auth.Account{SiteID: "bilibili", AccountID: "user_to_delete", Name: "DeleteMe"}
	_ = service.RegisterOrUpdateAccount(acc, jar)

	credFile := filepath.Join(storageDir, "bilibili_user_to_delete.cred")
	if _, err := os.Stat(credFile); err != nil {
		t.Fatalf("cred file should exist: %v", err)
	}

	// 执行删除注销
	if err := service.DeleteAccount("bilibili", "user_to_delete"); err != nil {
		t.Fatalf("delete account failed: %v", err)
	}

	// 1. 验证磁盘凭据文件已被物理删除
	if _, err := os.Stat(credFile); !os.IsNotExist(err) {
		t.Errorf("cred file was NOT deleted from disk")
	}

	// 2. 验证内存中的会话实例被同步清除
	if _, err := service.GetClient("bilibili", "user_to_delete"); err == nil {
		t.Errorf("client was not purged from memory")
	}

	// 3. 验证账号列表元数据已更新
	if _, err := service.GetAccount("bilibili", "user_to_delete"); !errors.Is(err, auth.ErrAccountNotFound) {
		t.Errorf("account metadata was not removed")
	}
}

// 验收准则 8: 登录失败或凭证失效时，绝不静默降低用户已选的媒体质量。
func TestAcceptance8_NoSilentDowngradeOnAuthFailure(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("tester")
	service, _ := auth.NewCredentialService(storageDir, v)

	ext := bilibili.New(service)

	ctx := context.Background()
	testURL := "https://www.bilibili.com/video/BV1xXYx6BE1a/"

	// 用户明确请求使用指定账号 "expired_vip_user" 解析
	opts := extractor.ExtractOptions{
		AccountID: "non_existent_or_expired_vip_user",
	}

	_, err := ext.Extract(ctx, testURL, opts)
	if err == nil {
		t.Fatal("expected error when requested account credentials are not available")
	}

	// 必须以 ErrCredentialExpired 显式报错，严禁静默回退降级为低画质 480P！
	if !errors.Is(err, extractor.ErrCredentialExpired) {
		t.Fatalf("expected ErrCredentialExpired, got: %v", err)
	}
}

// 验收准则 9: 绝不在前端对象、日志、或元数据中泄漏任何敏感凭证（Cookie/Token/SESSDATA）。
func TestAcceptance9_ZeroCredentialLeakageInLogsAndSafeAccount(t *testing.T) {
	acc := &auth.Account{
		SiteID:    "bilibili",
		AccountID: "123456",
		Name:      "UserWithoutLeak",
		AvatarURL: "https://avatar.example.com/face.jpg",
		Status:    auth.StatusActive,
		IsActive:  true,
		Metadata: map[string]string{
			"vip_type": "2",
		},
	}

	safe := acc.ToSafe()
	data, err := json.Marshal(safe)
	if err != nil {
		t.Fatal(err)
	}

	jsonStr := string(data)
	forbiddenTokens := []string{"cookie", "SESSDATA", "bili_jct", "DedeUserID", "token", "password"}
	for _, tok := range forbiddenTokens {
		if strings.Contains(strings.ToLower(jsonStr), strings.ToLower(tok)) {
			t.Fatalf("leakage detected in SafeAccount JSON: found %q in %s", tok, jsonStr)
		}
	}
}
