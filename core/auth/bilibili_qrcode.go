package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"media-fetcher/pkg/cookiejar"
)

// QRLoginState 表示哔哩哔哩二维码登录流程的状态枚举。
type QRLoginState string

const (
	QRStateWaiting   QRLoginState = "waiting"   // 86101: 二维码已生成，等待扫码
	QRStateScanned   QRLoginState = "scanned"   // 86090: 手机端已扫码，等待用户确认
	QRStateConfirmed QRLoginState = "confirmed" // 0: 手机端确认登录成功
	QRStateExpired   QRLoginState = "expired"   // 86038: 二维码已超时过期
	QRStateFailed    QRLoginState = "failed"    // 其他接口错误或网络异常
)

// QRCodeSession 维护当前活跃的二维码登录会话状态。
type QRCodeSession struct {
	QRCodeKey string       `json:"qrcode_key"`
	URL       string       `json:"url"` // 二维码原始目标 URL
	State     QRLoginState `json:"state"`
	Message   string       `json:"message"`
	CreatedAt time.Time    `json:"created_at"`
}

// BilibiliQRLoginCoordinator 协调哔哩哔哩扫码登录的完整生命周期。
type BilibiliQRLoginCoordinator struct {
	passportBase string
	apiBase      string
	credService  *CredentialService
	httpClient   *http.Client
}

// NewBilibiliQRLoginCoordinator 创建新的扫码登录协调器实例。
func NewBilibiliQRLoginCoordinator(credService *CredentialService) *BilibiliQRLoginCoordinator {
	return &BilibiliQRLoginCoordinator{
		passportBase: "https://passport.bilibili.com",
		apiBase:      "https://api.bilibili.com",
		credService:  credService,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

// SetBaseURLs 允许在单元测试中覆盖目标端点以重定向到本地 Mock 服务。
func (c *BilibiliQRLoginCoordinator) SetBaseURLs(passportBase, apiBase string) {
	c.passportBase = passportBase
	c.apiBase = apiBase
}

// GenerateQRCode 向哔哩哔哩请求创建新的二维码登录会话。
func (c *BilibiliQRLoginCoordinator) GenerateQRCode(ctx context.Context) (*QRCodeSession, error) {
	reqURL := fmt.Sprintf("%s/x/passport-login/web/qrcode/generate", strings.TrimRight(c.passportBase, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("bilibili qrcode: build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bilibili qrcode: network request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var reply struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			URL       string `json:"url"`
			QRCodeKey string `json:"qrcode_key"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &reply); err != nil {
		return nil, fmt.Errorf("bilibili qrcode: parse response: %w", err)
	}
	if reply.Code != 0 {
		return nil, fmt.Errorf("bilibili qrcode: API error code=%d msg=%s", reply.Code, reply.Message)
	}

	return &QRCodeSession{
		QRCodeKey: reply.Data.QRCodeKey,
		URL:       reply.Data.URL,
		State:     QRStateWaiting,
		Message:   "等待扫码",
		CreatedAt: time.Now(),
	}, nil
}

// PollResult 表示一次轮询检查的结果状态。
type PollResult struct {
	State       QRLoginState `json:"state"`
	Message     string       `json:"message"`
	AccountInfo *SafeAccount `json:"account_info,omitempty"`
	RawCode     int          `json:"raw_code"`
}

// PollStatus 查询哔哩哔哩服务端以获取当前二维码会话的状态。
// 当状态变为 QRStateConfirmed 时，自动采集 Set-Cookie 头部至新 Jar，
// 并请求 /x/web-interface/nav 核验用户身份，将已核验账号登记入凭证中心。
func (c *BilibiliQRLoginCoordinator) PollStatus(ctx context.Context, qrcodeKey string) (*PollResult, error) {
	pollURL := fmt.Sprintf("%s/x/passport-login/web/qrcode/poll?qrcode_key=%s",
		strings.TrimRight(c.passportBase, "/"), url.QueryEscape(qrcodeKey))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &PollResult{State: QRStateFailed, Message: "网络请求失败"}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var reply struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		return nil, fmt.Errorf("bilibili poll: parse response: %w", err)
	}

	code := reply.Data.Code
	// 当外层 code != 0 时，检查外层错误码
	if reply.Code != 0 {
		code = reply.Code
	}

	switch code {
	case 86101:
		return &PollResult{State: QRStateWaiting, Message: "等待扫码", RawCode: code}, nil
	case 86090:
		return &PollResult{State: QRStateScanned, Message: "已扫码，请在手机端确认", RawCode: code}, nil
	case 86038:
		return &PollResult{State: QRStateExpired, Message: "二维码已过期", RawCode: code}, nil
	case 0:
		// 扫码确认成功，收集 Cookie 凭据
		cookies := resp.Cookies()
		// 若返回重定向 URL，同步提取 URL 中的凭证参数
		if reply.Data.URL != "" {
			if u, err := url.Parse(reply.Data.URL); err == nil {
				q := u.Query()
				if sess := q.Get("SESSDATA"); sess != "" {
					cookies = append(cookies, &http.Cookie{
						Name:     "SESSDATA",
						Value:    sess,
						Domain:   ".bilibili.com",
						Path:     "/",
						HttpOnly: true,
						Secure:   true,
					})
				}
				if jct := q.Get("bili_jct"); jct != "" {
					cookies = append(cookies, &http.Cookie{
						Name:   "bili_jct",
						Value:  jct,
						Domain: ".bilibili.com",
						Path:   "/",
					})
				}
				if dede := q.Get("DedeUserID"); dede != "" {
					cookies = append(cookies, &http.Cookie{
						Name:   "DedeUserID",
						Value:  dede,
						Domain: ".bilibili.com",
						Path:   "/",
					})
				}
			}
		}

		if len(cookies) == 0 {
			return &PollResult{State: QRStateFailed, Message: "登录确认成功但未收到凭证 Cookie"}, errors.New("no cookies in response")
		}

		// 暂存至临时 Jar
		tempJar := cookiejar.New("bilibili", "pending_login")
		biliURL, _ := url.Parse("https://bilibili.com")
		tempJar.SetCookies(biliURL, cookies)

		// 调用 /x/web-interface/nav 进行在线身份验证
		verifyClient := &http.Client{Jar: tempJar, Timeout: 15 * time.Second}
		name, avatar, mid, valid, meta, err := c.verifyNav(ctx, verifyClient)
		if err != nil || !valid {
			return &PollResult{State: QRStateFailed, Message: "凭证身份校验未通过"}, fmt.Errorf("nav validation failed: %w", err)
		}

		accountID := strconv.FormatInt(mid, 10)
		finalJar := cookiejar.New("bilibili", accountID)
		finalJar.SetCookies(biliURL, cookies)

		acc := &Account{
			SiteID:          "bilibili",
			AccountID:       accountID,
			Name:            name,
			AvatarURL:       avatar,
			Status:          StatusActive,
			CreatedAt:       time.Now(),
			LastValidatedAt: time.Now(),
			Metadata:        meta,
		}

		if err := c.credService.RegisterOrUpdateAccount(acc, finalJar); err != nil {
			return &PollResult{State: QRStateFailed, Message: "保存凭证失败"}, err
		}

		safe := acc.ToSafe()
		return &PollResult{
			State:       QRStateConfirmed,
			Message:     "登录成功",
			AccountInfo: &safe,
			RawCode:     code,
		}, nil

	default:
		return &PollResult{
			State:   QRStateFailed,
			Message: fmt.Sprintf("扫码失败 (code=%d: %s)", code, reply.Data.Message),
			RawCode: code,
		}, fmt.Errorf("bilibili qr login failed: code=%d", code)
	}
}

// verifyNav 请求哔哩哔哩 nav 接口核验当前会话有效性并获取用户基本资料。
func (c *BilibiliQRLoginCoordinator) verifyNav(ctx context.Context, client *http.Client) (name, avatar string, mid int64, valid bool, meta map[string]string, err error) {
	navURL := fmt.Sprintf("%s/x/web-interface/nav", strings.TrimRight(c.apiBase, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, navURL, nil)
	if err != nil {
		return "", "", 0, false, nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", 0, false, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", 0, false, nil, err
	}

	var reply struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			IsLogin   bool   `json:"isLogin"`
			Mid       int64  `json:"mid"`
			Uname     string `json:"uname"`
			Face      string `json:"face"`
			VipType   int    `json:"vipType"`
			VipStatus int    `json:"vipStatus"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		return "", "", 0, false, nil, err
	}

	if reply.Code != 0 || !reply.Data.IsLogin || reply.Data.Mid <= 0 {
		return "", "", 0, false, nil, nil
	}

	metadata := map[string]string{
		"mid":        strconv.FormatInt(reply.Data.Mid, 10),
		"vip_type":   strconv.Itoa(reply.Data.VipType),
		"vip_status": strconv.Itoa(reply.Data.VipStatus),
	}

	return reply.Data.Uname, reply.Data.Face, reply.Data.Mid, true, metadata, nil
}
