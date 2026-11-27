package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"media-fetcher/pkg/vault"
)

type pgcDiagnosticTransport struct {
	base http.RoundTripper
	t    *testing.T
}

// 仅输出结构化白名单，响应正文、身份及 Cookie 值不得进入日志。
func pgcDiagnosticSummary(body []byte) map[string]any {
	var r struct {
		Code *int `json:"code"`
		Data struct {
			IsLogin   *bool `json:"isLogin"`
			VipType   *int  `json:"vipType"`
			VipStatus *int  `json:"vipStatus"`
			VIP       struct {
				Type   *int `json:"type"`
				Status *int `json:"status"`
			} `json:"vip"`
		} `json:"data"`
		Result struct {
			UserStatus struct {
				Login *int `json:"login"`
				Vip   *int `json:"vip"`
				Pay   *int `json:"pay"`
			} `json:"user_status"`
			VideoInfo struct {
				Code    *int   `json:"code"`
				Preview *int   `json:"is_preview"`
				DRM     *bool  `json:"is_drm"`
				Paid    *bool  `json:"has_paid"`
				Reason  *int   `json:"can_watch_reason"`
				Quality *int   `json:"quality"`
				Length  *int64 `json:"timelength"`
				Dash    struct {
					Video []json.RawMessage `json:"video"`
					Audio []json.RawMessage `json:"audio"`
				} `json:"dash"`
			} `json:"video_info"`
			PlayCheck struct {
				Detail string `json:"play_detail"`
			} `json:"play_check"`
		} `json:"result"`
	}
	if json.Unmarshal(body, &r) != nil {
		return map[string]any{"response_shape": "unexpected"}
	}
	detail := r.Result.PlayCheck.Detail
	switch detail {
	case "", "PLAY_WHOLE", "PLAY_PREVIEW", "PLAY_NONE":
	default:
		detail = "other"
	}
	v := r.Result.VideoInfo
	return map[string]any{"code": r.Code, "isLogin": r.Data.IsLogin, "vipType": r.Data.VipType, "vipStatus": r.Data.VipStatus, "vip.type": r.Data.VIP.Type, "vip.status": r.Data.VIP.Status, "season.login": r.Result.UserStatus.Login, "season.vip": r.Result.UserStatus.Vip, "season.pay": r.Result.UserStatus.Pay, "video.code": v.Code, "is_preview": v.Preview, "is_drm": v.DRM, "has_paid": v.Paid, "can_watch_reason": v.Reason, "quality": v.Quality, "timelength": v.Length, "play_detail": detail, "video_count": len(v.Dash.Video), "audio_count": len(v.Dash.Audio)}
}

func (d *pgcDiagnosticTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	count := 0
	for _, c := range req.Cookies() {
		if c.Name == "SESSDATA" {
			count++
		}
	}
	response, err := d.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	response.Body.Close()
	if err != nil {
		return nil, err
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	summary := pgcDiagnosticSummary(body)
	summary["path"] = req.URL.Path
	summary["http_status"] = response.StatusCode
	summary["sessdata_count"] = count
	summary["has_referer"] = req.Header.Get("Referer") != ""
	encoded, _ := json.Marshal(summary)
	d.t.Log(string(encoded))
	return response, nil
}

// 手动启用，仅使用当前活动账号；普通单元测试不访问网络和用户凭证。
func TestBilibiliPGCDiagnostic(t *testing.T) {
	if os.Getenv("MEDIA_FETCHER_PGC_DIAGNOSTIC") != "1" {
		t.Skip("手动脱敏诊断未启用")
	}
	service, err := NewCredentialService("", vault.NewDefaultVault())
	if err != nil {
		t.Fatal("无法读取本地凭证")
	}
	var selected *SafeAccount
	for _, a := range service.ListAccounts("bilibili") {
		if a.IsActive && a.Status == StatusActive {
			if selected != nil {
				t.Fatal("活动账号不唯一")
			}
			selected = &a
		}
	}
	if selected == nil {
		t.Fatal("没有可用活动账号，请在应用中重新扫码后再运行")
	}
	client, err := service.GetClient("bilibili", selected.AccountID)
	if err != nil {
		t.Fatal("账号客户端不可用")
	}
	transport := &pgcDiagnosticTransport{base: client.Transport, t: t}
	client.Transport = transport
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	coordinator := NewBilibiliQRLoginCoordinator(service)
	_, _, mid, valid, _, err := coordinator.verifyNav(ctx, client)
	if err != nil {
		t.Fatal("nav 请求或响应解析失败（未输出原始错误）")
	}
	t.Logf("nav_valid=%t account_matches=%t", valid, strconv.FormatInt(mid, 10) == selected.AccountID)
	if !valid {
		return
	}
	referer := "https://www.bilibili.com/bangumi/play/ep748436/"
	get := func(endpoint string) []byte {
		req, _ := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		req.Header.Set("Referer", referer)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal("PGC 请求失败（未输出原始错误）")
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		if err != nil {
			t.Fatal("读取 PGC 响应失败")
		}
		return body
	}
	body := get("https://api.bilibili.com/pgc/view/web/season?ep_id=748436")
	var season struct {
		Result struct {
			Episodes []struct {
				ID  int64 `json:"id"`
				CID int64 `json:"cid"`
			} `json:"episodes"`
		} `json:"result"`
	}
	if json.Unmarshal(body, &season) != nil {
		t.Fatal("季度信息格式异常")
	}
	cid := int64(0)
	for _, ep := range season.Result.Episodes {
		if ep.ID == 748436 {
			cid = ep.CID
		}
	}
	if cid == 0 {
		t.Fatal("季度中找不到目标 EP/CID")
	}
	get("https://api.bilibili.com/pgc/player/web/v2/playurl?ep_id=748436&cid=" + strconv.FormatInt(cid, 10) + "&fnval=16&fnver=0")
}

func TestPGCDiagnosticRedactsSecrets(t *testing.T) {
	body := []byte(`{"code":0,"message":"secret-token","result":{"play_check":{"play_detail":"secret-token"},"video_info":{"is_preview":1,"dash":{"video":[{"base_url":"https://secret-token/"}]}}}}`)
	encoded, _ := json.Marshal(pgcDiagnosticSummary(body))
	if bytes.Contains(encoded, []byte("secret-token")) {
		t.Fatal("脱敏摘要泄露响应内容")
	}
}
