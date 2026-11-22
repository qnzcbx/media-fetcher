package auth

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"media-fetcher/pkg/cookiejar"
	"media-fetcher/pkg/vault"
)

func TestCredentialService_IsolationAndPersistence(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("test_user")

	service, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatalf("failed to init service: %v", err)
	}

	biliURL, _ := url.Parse("https://bilibili.com")

	// 登记账号 A
	jarA := cookiejar.New("bilibili", "user_a")
	jarA.SetCookies(biliURL, []*http.Cookie{
		{Name: "SESSDATA", Value: "token_for_user_a", Domain: ".bilibili.com", Path: "/"},
	})
	accA := &Account{
		SiteID:    "bilibili",
		AccountID: "user_a",
		Name:      "Alice",
	}
	if err := service.RegisterOrUpdateAccount(accA, jarA); err != nil {
		t.Fatalf("register Alice failed: %v", err)
	}

	// 登记账号 B
	jarB := cookiejar.New("bilibili", "user_b")
	jarB.SetCookies(biliURL, []*http.Cookie{
		{Name: "SESSDATA", Value: "token_for_user_b", Domain: ".bilibili.com", Path: "/"},
	})
	accB := &Account{
		SiteID:    "bilibili",
		AccountID: "user_b",
		Name:      "Bob",
	}
	if err := service.RegisterOrUpdateAccount(accB, jarB); err != nil {
		t.Fatalf("register Bob failed: %v", err)
	}

	// 1. 验证客户端隔离
	clientA, err := service.GetClient("bilibili", "user_a")
	if err != nil {
		t.Fatalf("get client A: %v", err)
	}
	cookiesA := clientA.Jar.Cookies(biliURL)
	if len(cookiesA) != 1 || cookiesA[0].Value != "token_for_user_a" {
		t.Fatalf("unexpected cookies for A: %v", cookiesA)
	}

	clientB, err := service.GetClient("bilibili", "user_b")
	if err != nil {
		t.Fatalf("get client B: %v", err)
	}
	cookiesB := clientB.Jar.Cookies(biliURL)
	if len(cookiesB) != 1 || cookiesB[0].Value != "token_for_user_b" {
		t.Fatalf("unexpected cookies for B: %v", cookiesB)
	}

	// 2. 验证匿名客户端隔离（绝不能包含任何已保存的 Cookie 凭据）
	anonClient := service.GetAnonymousClient("bilibili")
	anonCookies := anonClient.Jar.Cookies(biliURL)
	if len(anonCookies) != 0 {
		t.Fatalf("anonymous client must NOT have any cookies, got: %v", anonCookies)
	}

	// 3. 验证当前活动账号切换
	active, err := service.GetActiveAccount("bilibili")
	if err != nil || active.AccountID != "user_a" {
		t.Fatalf("expected Alice to be active initially, got: %+v", active)
	}

	if err := service.SetActiveAccount("bilibili", "user_b"); err != nil {
		t.Fatalf("set active account failed: %v", err)
	}
	active, err = service.GetActiveAccount("bilibili")
	if err != nil || active.AccountID != "user_b" {
		t.Fatalf("expected Bob to be active, got: %+v", active)
	}

	// 4. 验证重启与凭据恢复
	serviceRestarted, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatalf("restart service failed: %v", err)
	}

	restoredB, err := serviceRestarted.GetClient("bilibili", "user_b")
	if err != nil {
		t.Fatalf("restored get client B: %v", err)
	}
	restoredCookiesB := restoredB.Jar.Cookies(biliURL)
	if len(restoredCookiesB) != 1 || restoredCookiesB[0].Value != "token_for_user_b" {
		t.Fatalf("restored cookies for B mismatch: %v", restoredCookiesB)
	}

	// 5. 验证账号注销
	credFileA := service.credFilePath("bilibili", "user_a")
	if _, err := os.Stat(credFileA); err != nil {
		t.Fatalf("credential file for A should exist before delete: %v", err)
	}

	if err := service.DeleteAccount("bilibili", "user_a"); err != nil {
		t.Fatalf("delete account A failed: %v", err)
	}

	// 磁盘加密文件必须被物理删除
	if _, err := os.Stat(credFileA); !os.IsNotExist(err) {
		t.Fatalf("credential file for A should be deleted, got: %v", err)
	}

	// 注销后不能再获取客户端
	if _, err := service.GetClient("bilibili", "user_a"); err == nil {
		t.Fatal("expected error getting deleted account client")
	}
}

func TestCredentialService_CorruptFileHandling(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("test_user")

	service, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	jar := cookiejar.New("bilibili", "corrupt_user")
	acc := &Account{SiteID: "bilibili", AccountID: "corrupt_user", Name: "Corrupt"}
	if err := service.RegisterOrUpdateAccount(acc, jar); err != nil {
		t.Fatal(err)
	}

	// 故意破坏加密凭证文件内容
	credFile := service.credFilePath("bilibili", "corrupt_user")
	_ = os.WriteFile(credFile, []byte("totally_corrupt_data"), 0600)

	// 重启凭证服务 -> 绝不能 panic，平稳标记状态为 expired
	restarted, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatalf("service restart with corrupt file should not error out: %v", err)
	}

	restoredAcc, err := restarted.GetAccount("bilibili", "corrupt_user")
	if err != nil {
		t.Fatal(err)
	}
	if restoredAcc.Status != StatusExpired {
		t.Fatalf("expected status expired for corrupt file, got: %s", restoredAcc.Status)
	}
}

// H1 回归测试：HTTP 重定向跨站凭据剥离保护
func TestH1_RedirectCrossHostCookieStripping(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("test_user")
	service, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	var untrustedReceivedCookie string

	// 不受信任的第三方目标服务器
	untrustedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		untrustedReceivedCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusOK)
	}))
	defer untrustedServer.Close()

	// 站内源服务器，发起 302 重定向到不受信任的第三方域名 http://evil.com/target
	trustedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://evil.com/target", http.StatusFound)
	}))
	defer trustedServer.Close()

	srvURL, _ := url.Parse(trustedServer.URL)
	jar := cookiejar.New("bilibili", "redirect_user")
	jar.SetCookies(srvURL, []*http.Cookie{
		{Name: "SESSDATA", Value: "secret_token_val", Path: "/"},
	})
	_ = service.RegisterOrUpdateAccount(&Account{SiteID: "bilibili", AccountID: "redirect_user", Name: "RedirUser"}, jar)

	client, err := service.GetClient("bilibili", "redirect_user")
	if err != nil {
		t.Fatal(err)
	}

	// 拦截 evil.com 并将其网络连接路由到本地 untrustedServer，用于模拟真实域名跨站重定向
	dialer := &net.Dialer{}
	customTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.HasPrefix(addr, "evil.com:") {
				return dialer.DialContext(ctx, network, untrustedServer.Listener.Addr().String())
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	client.Transport = &revocableTransport{
		base:   customTransport,
		jar:    jar,
		siteID: "bilibili",
	}

	resp, err := client.Get(trustedServer.URL)
	if err != nil {
		t.Fatalf("请求重定向链路失败: %v", err)
	}
	defer resp.Body.Close()

	// 验证不受信任的第三方服务器绝未收到该账号的敏感 Cookie
	if untrustedReceivedCookie != "" {
		t.Fatalf("严重安全漏洞：重定向到第三方站点时泄露了 Cookie: %s", untrustedReceivedCookie)
	}
}

// H2 回归测试：服务端在 HTTP 响应中更新与删除 Cookie 必须自动持久化并在重启后生效
func TestH2_AutoPersistOnRuntimeSetCookie(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("test_user")
	service, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	currentToken := "token_v1"
	isDeleteRequested := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isDeleteRequested {
			// 服务端通知客户端删除 Cookie
			http.SetCookie(w, &http.Cookie{
				Name:    "SESSDATA",
				Value:   "",
				Path:    "/",
				MaxAge:  -1,
				Expires: time.Now().Add(-1 * time.Hour),
			})
		} else {
			// 服务端轮换更新 Cookie
			http.SetCookie(w, &http.Cookie{
				Name:  "SESSDATA",
				Value: currentToken,
				Path:  "/",
			})
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	srvURL, _ := url.Parse(server.URL)
	jar := cookiejar.New("bilibili", "persist_user")
	jar.SetCookies(srvURL, []*http.Cookie{
		{Name: "SESSDATA", Value: "initial_token", Path: "/"},
	})
	acc := &Account{SiteID: "bilibili", AccountID: "persist_user", Name: "AutoPersistUser"}
	if err := service.RegisterOrUpdateAccount(acc, jar); err != nil {
		t.Fatal(err)
	}

	client, err := service.GetClient("bilibili", "persist_user")
	if err != nil {
		t.Fatal(err)
	}

	// 1. 发起请求，服务端下发轮换后的 token_v1
	currentToken = "token_v1_rotated"
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// 模拟进程重启（不显式调用 Save，直接通过新 Service 实例读取原目录）
	service = nil
	restartedService, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatalf("重启失败: %v", err)
	}

	restartedClient, err := restartedService.GetClient("bilibili", "persist_user")
	if err != nil {
		t.Fatal(err)
	}
	cookies := restartedClient.Jar.Cookies(srvURL)
	if len(cookies) != 1 || cookies[0].Value != "token_v1_rotated" {
		t.Fatalf("H2 缺陷：服务端更新的 Cookie 在重启后未能自动持久化恢复！实际得到: %+v", cookies)
	}

	// 2. 发起请求，服务端下发删除 Cookie 指令
	isDeleteRequested = true
	resp2, err := restartedClient.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()

	// 再次模拟重启
	restartedService2, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}
	restartedClient2, err := restartedService2.GetClient("bilibili", "persist_user")
	if err != nil {
		t.Fatal(err)
	}
	cookiesAfterDelete := restartedClient2.Jar.Cookies(srvURL)
	if len(cookiesAfterDelete) != 0 {
		t.Fatalf("H2 缺陷：服务端删除的 Cookie 在重启后发生回退幽灵恢复！实际仍存在: %+v", cookiesAfterDelete)
	}
}

// H3 回归测试：账号注销必须主动撤销旧 Client 引用，且磁盘删除失败必须显式报错
func TestH3_RevokeOldClientOnDeleteAccount(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("test_user")
	service, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	srvURL, _ := url.Parse(server.URL)
	jar := cookiejar.New("bilibili", "revoke_user")
	jar.SetCookies(srvURL, []*http.Cookie{{Name: "SESSDATA", Value: "val", Path: "/"}})
	acc := &Account{SiteID: "bilibili", AccountID: "revoke_user", Name: "RevokeUser"}
	_ = service.RegisterOrUpdateAccount(acc, jar)

	// 事先获取 Client 句柄
	client, err := service.GetClient("bilibili", "revoke_user")
	if err != nil {
		t.Fatal(err)
	}

	// 执行注销
	if err := service.DeleteAccount("bilibili", "revoke_user"); err != nil {
		t.Fatalf("注销账号失败: %v", err)
	}

	// 验证持有旧 Client 发起网络请求会被主动撤销拦截
	_, err = client.Get(server.URL)
	if err == nil {
		t.Fatal("严重缺陷：账号已注销，但持有的旧 Client 仍能成功发出网络请求！")
	}
	if !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("期望得到 ErrSessionRevoked, 实际得到: %v", err)
	}

	// 验证旧 Jar 拒绝一切写入与读取
	if len(jar.Cookies(srvURL)) != 0 {
		t.Fatal("已撤销的旧 Jar 应当返回 0 Cookie")
	}
	jar.SetCookies(srvURL, []*http.Cookie{{Name: "new_cookie", Value: "sneak", Path: "/"}})
	if len(jar.Cookies(srvURL)) != 0 {
		t.Fatal("已撤销的旧 Jar 绝不能允许写入新 Cookie")
	}
}

// H3 回归测试：磁盘凭证删除失败必须返回显式错误，绝不能被吞掉
func TestH3_DeleteAccountDiskErrorReported(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("test_user")
	service, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	jar := cookiejar.New("bilibili", "fail_delete_user")
	acc := &Account{SiteID: "bilibili", AccountID: "fail_delete_user", Name: "FailUser"}
	_ = service.RegisterOrUpdateAccount(acc, jar)

	credFile := service.credFilePath("bilibili", "fail_delete_user")

	// 在 Windows 上，独占打开文件以模拟文件被锁定无法删除的情况
	lockedFile, err := os.OpenFile(credFile, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lockedFile.Close()

	err = service.DeleteAccount("bilibili", "fail_delete_user")
	if err == nil {
		t.Fatal("H3 缺陷：磁盘删除失败被静默吞掉，未向调用方返回错误！")
	}
	if !strings.Contains(err.Error(), "删除磁盘凭证文件失败") {
		t.Fatalf("期望包含删除失败错误提示，实际为: %v", err)
	}
}
