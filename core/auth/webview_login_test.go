package auth

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"media-fetcher/pkg/vault"
)

func TestWebViewLoginCoordinator_Lifecycle(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("test_user")
	service, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	coord := NewWebViewLoginCoordinator(service)

	// 1. 开启会话
	sess, err := coord.StartSession("bilibili")
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	if sess.SessionID == "" || sess.LoginURL == "" {
		t.Fatalf("invalid session returned: %+v", sess)
	}

	// 2. 取消会话测试
	sessCancel, err := coord.StartSession("bilibili")
	if err != nil {
		t.Fatal(err)
	}
	if err := coord.CancelSession(sessCancel.SessionID); err != nil {
		t.Fatalf("CancelSession failed: %v", err)
	}
	// 尝试完成已取消的会话必须失败
	_, err = coord.CompleteSession(context.Background(), sessCancel.SessionID, []*http.Cookie{
		{Name: "SESSDATA", Value: "val"},
	}, nil)
	if err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound for cancelled session, got: %v", err)
	}

	// 3. 携带核验函数完成登录会话
	mockCookies := []*http.Cookie{
		{
			Name:     "SESSDATA",
			Value:    "webview_secret_token",
			Domain:   ".bilibili.com",
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
		},
	}

	validator := func(ctx context.Context, client *http.Client) (name, avatar, accountID string, valid bool, meta map[string]string, err error) {
		// 校验核验客户端已成功挂载临时 Cookie
		u, _ := url.Parse("https://bilibili.com")
		c := client.Jar.Cookies(u)
		if len(c) == 0 || c[0].Value != "webview_secret_token" {
			return "", "", "", false, nil, nil
		}
		return "WebUser", "https://avatar.example.com/1.png", "555888", true, map[string]string{"type": "vip"}, nil
	}

	safeAcc, err := coord.CompleteSession(context.Background(), sess.SessionID, mockCookies, validator)
	if err != nil {
		t.Fatalf("CompleteSession failed: %v", err)
	}
	if safeAcc.AccountID != "555888" || safeAcc.Name != "WebUser" {
		t.Fatalf("unexpected safe account: %+v", safeAcc)
	}

	// 验证在 CredentialService 中完成加密持久化登记
	acc, err := service.GetAccount("bilibili", "555888")
	if err != nil || acc.Name != "WebUser" {
		t.Fatalf("account not registered in service: %v", err)
	}

	client, err := service.GetClient("bilibili", "555888")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse("https://bilibili.com")
	cookies := client.Jar.Cookies(u)
	if len(cookies) == 0 || cookies[0].Value != "webview_secret_token" {
		t.Fatalf("client cookies mismatch: %v", cookies)
	}
}
