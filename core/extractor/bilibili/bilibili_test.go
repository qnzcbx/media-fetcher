package bilibili

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"media-fetcher/core/auth"
	"media-fetcher/core/extractor"
	"media-fetcher/pkg/cookiejar"
	"media-fetcher/pkg/vault"
)

func TestBilibiliExtractor_AnonymousAndAuthorized(t *testing.T) {
	storageDir := t.TempDir()
	v := vault.NewMockVault("test_user")
	credService, err := auth.NewCredentialService(storageDir, v)
	if err != nil {
		t.Fatal(err)
	}

	// 配置会员账号 "vip_user"
	jar := cookiejar.New("bilibili", "vip_user")
	acc := &auth.Account{SiteID: "bilibili", AccountID: "vip_user", Name: "VIP Member"}
	_ = credService.RegisterOrUpdateAccount(acc, jar)

	// 本地 Mock 服务端
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/web-interface/view":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"data": map[string]interface{}{
					"bvid":  "BV1xXYx6BE1a",
					"cid":   123456,
					"title": "测试视频",
					"pic":   "https://i0.hdslb.com/bfs/archive/cover.jpg",
					"owner": map[string]interface{}{"name": "UP主"},
					"pages": []map[string]interface{}{{"cid": 123456, "page": 1}},
				},
			})

		case "/x/web-interface/nav":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"data": map[string]interface{}{
					"wbi_img": map[string]string{
						"img_url": "https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png",
						"sub_url": "https://i0.hdslb.com/bfs/wbi/4932caff0ff746eab6f01bf08b70ac45.png",
					},
				},
			})

		case "/x/player/wbi/playurl":
			hasVIPCookie := false
			for _, c := range r.Cookies() {
				if c.Name == "SESSDATA" && c.Value == "vip_token_xyz" {
					hasVIPCookie = true
					break
				}
			}

			// 若携带有效 VIP Cookie，则返回 1080P 和 480P；否则仅返回 480P
			var videoStreams []map[string]interface{}
			if hasVIPCookie {
				videoStreams = append(videoStreams, map[string]interface{}{
					"id":        80, // 1080P
					"width":     1920,
					"height":    1080,
					"codecs":    "avc1.640028",
					"bandwidth": 3000000,
					"baseUrl":   "https://cdn.example.com/1080p.m4s",
				})
			}
			videoStreams = append(videoStreams, map[string]interface{}{
				"id":        32, // 480P
				"width":     852,
				"height":    480,
				"codecs":    "avc1.64001F",
				"bandwidth": 800000,
				"baseUrl":   "https://cdn.example.com/480p.m4s",
			})

			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0,
				"data": map[string]interface{}{
					"dash": map[string]interface{}{
						"video": videoStreams,
						"audio": []map[string]interface{}{
							{
								"id":        30280,
								"codecs":    "mp4a.40.2",
								"bandwidth": 192000,
								"baseUrl":   "https://cdn.example.com/audio.m4s",
							},
						},
					},
				},
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// 将测试 VIP Cookie 绑定至 Mock 测试服务主机
	srvURL, _ := url.Parse(server.URL)
	jar.SetCookies(srvURL, []*http.Cookie{
		{Name: "SESSDATA", Value: "vip_token_xyz", Path: "/"},
	})
	_ = credService.RegisterOrUpdateAccount(acc, jar)

	ext := New(credService)
	ext.SetBaseURLs(server.URL, server.URL)

	ctx := context.Background()
	testURL := "https://www.bilibili.com/video/BV1xXYx6BE1a/"

	// 1. 匿名解析场景验证
	anonInfo, err := ext.Extract(ctx, testURL, extractor.ExtractOptions{})
	if err != nil {
		t.Fatalf("anonymous extract failed: %v", err)
	}
	if !anonInfo.IsAnonymous {
		t.Error("expected IsAnonymous=true")
	}
	if len(anonInfo.VideoStreams) != 1 || anonInfo.VideoStreams[0].ID != 32 {
		t.Fatalf("expected only 480P for anonymous, got: %+v", anonInfo.VideoStreams)
	}

	// 2. 携带 VIP 会员账号的认证解析场景验证
	authInfo, err := ext.Extract(ctx, testURL, extractor.ExtractOptions{AccountID: "vip_user"})
	if err != nil {
		t.Fatalf("auth extract failed: %v", err)
	}
	if authInfo.IsAnonymous {
		t.Error("expected IsAnonymous=false")
	}
	if authInfo.UsedAccountID != "vip_user" {
		t.Errorf("expected UsedAccountID=vip_user, got: %s", authInfo.UsedAccountID)
	}
	if len(authInfo.VideoStreams) != 2 || authInfo.VideoStreams[0].ID != 80 {
		t.Fatalf("expected 1080P stream for VIP, got: %+v", authInfo.VideoStreams)
	}

	// 3. 请求不存在或已失效账号时，必须返回 ErrCredentialExpired 且严禁静默降级！
	_, err = ext.Extract(ctx, testURL, extractor.ExtractOptions{AccountID: "non_existent_user"})
	if err == nil {
		t.Fatal("expected error when requested account is missing")
	}
	if !errors.Is(err, extractor.ErrCredentialExpired) {
		t.Fatalf("expected ErrCredentialExpired, got: %v", err)
	}
}

func TestBilibiliExtractor_Match(t *testing.T) {
	ext := New(nil)

	validURLs := []string{
		"https://www.bilibili.com/video/BV1xXYx6BE1a/",
		"http://bilibili.com/video/BV1xXYx6BE1a?p=2",
		"【标题】https://www.bilibili.com/video/BV1kAtz6QEun/?share_source=copy 复制分享",
	}
	for _, u := range validURLs {
		if !ext.Match(u) {
			t.Errorf("expected Match=true for %q", u)
		}
	}

	invalidURLs := []string{
		"https://www.youtube.com/watch?v=123",
		"https://www.bilibili.com/read/cv123",
		"https://bilibili.com/video/invalid_bv",
	}
	for _, u := range invalidURLs {
		if ext.Match(u) {
			t.Errorf("expected Match=false for %q", u)
		}
	}
}
