package bilibili

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"media-fetcher/core/auth"
	"media-fetcher/core/extractor"
	"media-fetcher/pkg/cookiejar"
	"media-fetcher/pkg/vault"
)

func TestPGCEpisodesAndSession(t *testing.T) {
	cred, err := auth.NewCredentialService(t.TempDir(), vault.NewMockVault("pgc"))
	if err != nil {
		t.Fatal(err)
	}
	jar := cookiejar.New("bilibili", "account")
	var playCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pgc/view/web/season":
			fmt.Fprint(w, `{"code":0,"result":{"title":"季度","cover":"cover","episodes":[{"id":1,"cid":11,"title":"1","long_title":"开始","duration":1000}],"section":[{"episodes":[{"id":2,"cid":22,"title":"特别篇","long_title":"结束"}]}]}}`)
		case "/pgc/player/web/v2/playurl":
			playCalls++
			id := r.URL.Query().Get("ep_id")
			cid := r.URL.Query().Get("cid")
			if (id == "1" && cid != "11") || (id == "2" && cid != "22") {
				t.Errorf("混用剧集: %s", r.URL)
			}
			token := "anon"
			if c, e := r.Cookie("SESSDATA"); e == nil {
				token = c.Value
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "result": map[string]any{"play_check": map[string]string{"play_detail": "PLAY_WHOLE"}, "video_info": map[string]any{"dash": map[string]any{"video": []map[string]any{{"id": 80, "codecs": "avc1", "base_url": "https://cdn.example/" + id + "/" + token}}, "audio": []map[string]any{{"id": 30280, "baseUrl": "https://cdn.example/audio/" + id}}}}}})
		default:
			t.Errorf("意外端点 %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	u, _ := http.NewRequest("GET", server.URL, nil)
	jar.SetCookies(u.URL, []*http.Cookie{{Name: "SESSDATA", Value: "selected"}})
	if err := cred.RegisterOrUpdateAccount(&auth.Account{SiteID: "bilibili", AccountID: "account", Name: "测试"}, jar); err != nil {
		t.Fatal(err)
	}
	e := New(cred)
	e.SetBaseURLs(server.URL, server.URL)
	for _, raw := range []string{"https://www.bilibili.com/bangumi/play/ep1/", "https://www.bilibili.com/bangumi/play/ss9"} {
		if !e.Match(raw) {
			t.Fatal(raw)
		}
	}
	season, err := e.Extract(context.Background(), "https://www.bilibili.com/bangumi/play/ss9", extractor.ExtractOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(season.Episodes) != 2 || len(season.VideoStreams) != 0 || playCalls != 0 {
		t.Fatalf("季度必须先选择: %+v", season)
	}
	for _, account := range []string{"account", ""} {
		for i, ep := range season.Episodes {
			m, err := e.Extract(context.Background(), ep.URL, extractor.ExtractOptions{AccountID: account})
			if err != nil {
				t.Fatal(err)
			}
			token := "anon"
			if account != "" {
				token = "selected"
			}
			if m.ID != fmt.Sprintf("ep%d", i+1) || m.VideoStreams[0].URL != fmt.Sprintf("https://cdn.example/%d/%s", i+1, token) || m.SourceURL != ep.URL || m.UsedAccountID != account || m.IsAnonymous != (account == "") || m.EpisodeTitle == "" {
				t.Fatalf("错误剧集或会话: %+v", m)
			}
		}
	}
	if _, err := e.Extract(context.Background(), episodeURL(3), extractor.ExtractOptions{}); !errors.Is(err, extractor.ErrAPIDenied) {
		t.Fatal(err)
	}
	if _, err := e.Extract(context.Background(), episodeURL(1), extractor.ExtractOptions{AccountID: "missing"}); !errors.Is(err, extractor.ErrCredentialExpired) {
		t.Fatal(err)
	}
}

func TestPGCRejectsRestrictedResources(t *testing.T) {
	for _, tc := range []struct {
		name, reply string
		want        error
	}{
		{"login", `{"code":-101,"message":"请登录"}`, extractor.ErrCredentialExpired},
		{"permission", `{"code":-10403,"message":"大会员或地区限制"}`, extractor.ErrAPIDenied},
		{"preview", `{"code":0,"result":{"play_check":{"play_detail":"PLAY_PREVIEW"}}}`, extractor.ErrAPIDenied},
		{"drm", `{"code":0,"result":{"video_info":{"is_drm":true}}}`, extractor.ErrUnsupportedMedia},
		{"non-dash", `{"code":0,"result":{"video_info":{"durl":[{}]}}}`, extractor.ErrUnsupportedMedia},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/pgc/view/web/season" {
					fmt.Fprint(w, `{"result":{"episodes":[{"id":1,"cid":11,"title":"1"}]}}`)
				} else {
					fmt.Fprint(w, tc.reply)
				}
			}))
			defer server.Close()
			e := New(nil)
			e.SetBaseURLs(server.URL, server.URL)
			if _, err := e.Extract(context.Background(), episodeURL(1), extractor.ExtractOptions{}); !errors.Is(err, tc.want) {
				t.Fatalf("%v", err)
			}
		})
	}
}
