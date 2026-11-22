package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	// Mock Bilibili Passport and API Server
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
				// State 1: Waiting
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code": 0,
					"data": map[string]interface{}{
						"code":    86101,
						"message": "未扫码",
					},
				})
			} else if pollCallCount == 2 {
				// State 2: Scanned
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code": 0,
					"data": map[string]interface{}{
						"code":    86090,
						"message": "已扫码",
					},
				})
			} else {
				// State 3: Confirmed with Cookie
				http.SetCookie(w, &http.Cookie{
					Name:     "SESSDATA",
					Value:    "sess_verified_token",
					Path:     "/",
					Domain:   ".bilibili.com",
					HttpOnly: true,
					Secure:   true,
				})
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code": 0,
					"data": map[string]interface{}{
						"code": 0,
						"url":  "https://passport.bilibili.com/crossDomain?SESSDATA=sess_verified_token",
					},
				})
			}

		case "/x/web-interface/nav":
			// Identity verification endpoint
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

	// 1. Generate QR Code
	qrSession, err := coord.GenerateQRCode(ctx)
	if err != nil {
		t.Fatalf("GenerateQRCode failed: %v", err)
	}
	if qrSession.QRCodeKey != "mock_key_123" {
		t.Fatalf("unexpected qrcode key: %s", qrSession.QRCodeKey)
	}

	// 2. Poll 1 -> Waiting
	res1, err := coord.PollStatus(ctx, qrSession.QRCodeKey)
	if err != nil || res1.State != QRStateWaiting {
		t.Fatalf("expected waiting, got: %+v, err: %v", res1, err)
	}

	// 3. Poll 2 -> Scanned
	res2, err := coord.PollStatus(ctx, qrSession.QRCodeKey)
	if err != nil || res2.State != QRStateScanned {
		t.Fatalf("expected scanned, got: %+v, err: %v", res2, err)
	}

	// 4. Poll 3 -> Confirmed & Auto-Registered!
	res3, err := coord.PollStatus(ctx, qrSession.QRCodeKey)
	if err != nil || res3.State != QRStateConfirmed {
		t.Fatalf("expected confirmed, got: %+v, err: %v", res3, err)
	}
	if res3.AccountInfo == nil || res3.AccountInfo.AccountID != "987654321" {
		t.Fatalf("expected registered account info, got: %+v", res3.AccountInfo)
	}

	// Verify Account is in CredentialService and client has cookie
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
	if len(cookies) == 0 || cookies[0].Value != "sess_verified_token" {
		t.Fatalf("cookies not set in client jar: %v", cookies)
	}
}
