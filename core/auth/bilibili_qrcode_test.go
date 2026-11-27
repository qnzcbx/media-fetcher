package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"media-fetcher/pkg/vault"
)

func TestBilibiliQRLoginCoordinator_FullFlow(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("test_user")
	service, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	pollCallCount := 0

	// 模拟哔哩哔哩 Passport 与 API 服务端
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/passport-login/web/qrcode/generate":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"data": map[string]interface{}{
					"url":        "https://passport.bilibili.com/scan?key=mock_key_123",
					"qrcode_key": "mock_key_123",
				},
			})

		case "/x/passport-login/web/qrcode/poll":
			pollCallCount++
			if pollCallCount == 1 {
				// 状态 1：等待扫码
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code": 0,
					"data": map[string]interface{}{
						"code":    86101,
						"message": "未扫码",
					},
				})
			} else if pollCallCount == 2 {
				// 状态 2：已扫码
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code": 0,
					"data": map[string]interface{}{
						"code":    86090,
						"message": "已扫码",
					},
				})
			} else {
				// 状态 3：确认登录并下发 Cookie
				http.SetCookie(w, &http.Cookie{
					Name:     "SESSDATA",
					Value:    "sess%2Cverified%2Ftoken",
					Path:     "/",
					Domain:   ".bilibili.com",
					HttpOnly: true,
					Secure:   true,
				})
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code": 0,
					"data": map[string]interface{}{
						"code": 0,
						"url":  "https://passport.bilibili.com/crossDomain?SESSDATA=sess%2Cverified%2Ftoken",
					},
				})
			}

		case "/x/web-interface/nav":
			// 身份核验接口
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"data": map[string]interface{}{
					"isLogin":   true,
					"mid":       987654321,
					"uname":     "BiliMaster",
					"face":      "https://i0.hdslb.com/bfs/face/test.jpg",
					"vipType":   2,
					"vipStatus": 1,
				},
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	coord := NewBilibiliQRLoginCoordinator(service)
	coord.SetBaseURLs(server.URL, server.URL)

	ctx := context.Background()

	// 1. 生成二维码
	qrSession, err := coord.GenerateQRCode(ctx)
	if err != nil {
		t.Fatalf("GenerateQRCode failed: %v", err)
	}
	if qrSession.QRCodeKey != "mock_key_123" {
		t.Fatalf("unexpected qrcode key: %s", qrSession.QRCodeKey)
	}

	// 2. 第 1 次轮询 -> 等待扫码
	res1, err := coord.PollStatus(ctx, qrSession.QRCodeKey)
	if err != nil || res1.State != QRStateWaiting {
		t.Fatalf("expected waiting, got: %+v, err: %v", res1, err)
	}

	// 3. 第 2 次轮询 -> 已扫码
	res2, err := coord.PollStatus(ctx, qrSession.QRCodeKey)
	if err != nil || res2.State != QRStateScanned {
		t.Fatalf("expected scanned, got: %+v, err: %v", res2, err)
	}

	// 4. 第 3 次轮询 -> 确认成功并自动完成登记
	res3, err := coord.PollStatus(ctx, qrSession.QRCodeKey)
	if err != nil || res3.State != QRStateConfirmed {
		t.Fatalf("expected confirmed, got: %+v, err: %v", res3, err)
	}
	if res3.AccountInfo == nil || res3.AccountInfo.AccountID != "987654321" {
		t.Fatalf("expected registered account info, got: %+v", res3.AccountInfo)
	}

	// 验证账号已存入 CredentialService 且 Client 携带有效 Cookie
	acc, err := service.GetAccount("bilibili", "987654321")
	if err != nil || acc.Name != "BiliMaster" {
		t.Fatalf("account not found in service: %+v, err: %v", acc, err)
	}

	client, err := service.GetClient("bilibili", "987654321")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := http.NewRequest("GET", "https://bilibili.com", nil)
	cookies := client.Jar.Cookies(u.URL)
	if len(cookies) == 0 || cookies[0].Value != "sess%2Cverified%2Ftoken" {
		t.Fatalf("cookies not set in client jar: %v", cookies)
	}
	restored, err := NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}
	restoredClient, err := restored.GetClient("bilibili", "987654321")
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range []*http.Client{client, restoredClient} {
		req, _ := http.NewRequest("GET", "https://api.bilibili.com/pgc/player/web/v2/playurl", nil)
		for _, cookie := range client.Jar.Cookies(req.URL) {
			req.AddCookie(cookie)
		}
		cookie, err := req.Cookie("SESSDATA")
		if err != nil || cookie.Value != "sess%2Cverified%2Ftoken" {
			t.Fatal("SESSDATA 未按原值持久化并发送")
		}
	}
}

func TestLoginSessionValuePreservesEscapes(t *testing.T) {
	for _, value := range []string{"plain", "test%2Csession%2Fvalue", "test%252Csession", "test+value"} {
		u, err := url.Parse("https://passport.bilibili.com/crossDomain?SESSDATA=" + value + "&bili_jct=test")
		if err != nil || loginSessionValue(u) != value {
			t.Fatal("登录 URL 的 SESSDATA 被改写")
		}
	}
}
