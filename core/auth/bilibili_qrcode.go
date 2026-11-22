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

// QRLoginState represents the state of a Bilibili QR code login process.
type QRLoginState string

const (
	QRStateWaiting   QRLoginState = "waiting"   // 86101: QR created, waiting to be scanned
	QRStateScanned   QRLoginState = "scanned"   // 86090: QR scanned by phone, waiting for confirmation
	QRStateConfirmed QRLoginState = "confirmed" // 0: User confirmed on phone, login succeeded
	QRStateExpired   QRLoginState = "expired"   // 86038: QR code expired
	QRStateFailed    QRLoginState = "failed"    // Other API errors or network failure
)

// QRCodeSession holds the state of an active QR code login attempt.
type QRCodeSession struct {
	QRCodeKey string       `json:"qrcode_key"`
	URL       string       `json:"url"` // QR code target URL
	State     QRLoginState `json:"state"`
	Message   string       `json:"message"`
	CreatedAt time.Time    `json:"created_at"`
}

// BilibiliQRLoginCoordinator handles the Bilibili QR login lifecycle.
type BilibiliQRLoginCoordinator struct {
	passportBase string
	apiBase      string
	credService  *CredentialService
	httpClient   *http.Client
}

// NewBilibiliQRLoginCoordinator creates a new coordinator.
func NewBilibiliQRLoginCoordinator(credService *CredentialService) *BilibiliQRLoginCoordinator {
	return &BilibiliQRLoginCoordinator{
		passportBase: "https://passport.bilibili.com",
		apiBase:      "https://api.bilibili.com",
		credService:  credService,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

// SetBaseURLs allows overriding endpoints for mock unit tests.
func (c *BilibiliQRLoginCoordinator) SetBaseURLs(passportBase, apiBase string) {
	c.passportBase = passportBase
	c.apiBase = apiBase
}

// GenerateQRCode requests a new QR code login session from Bilibili.
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

// PollResult represents the result of a single poll check.
type PollResult struct {
	State       QRLoginState `json:"state"`
	Message     string       `json:"message"`
	AccountInfo *SafeAccount `json:"account_info,omitempty"`
}

// PollStatus queries Bilibili for the current status of the QR code session.
// When state reaches QRStateConfirmed, it collects the Set-Cookie headers into a new jar,
// verifies identity against /x/web-interface/nav, and registers the verified account.
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
	// When top-level code != 0, check top-level
	if reply.Code != 0 {
		code = reply.Code
	}

	switch code {
	case 86101:
		return &PollResult{State: QRStateWaiting, Message: "等待扫码"}, nil
	case 86090:
		return &PollResult{State: QRStateScanned, Message: "已扫码，请在手机端确认"}, nil
	case 86038:
		return &PollResult{State: QRStateExpired, Message: "二维码已过期"}, nil
	case 0:
		// Confirmed! Collect cookies
		cookies := resp.Cookies()
		// Also parse cookies from redirected url if present
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

		// Stage into a temporary jar
		tempJar := cookiejar.New("bilibili", "pending_login")
		biliURL, _ := url.Parse("https://bilibili.com")
		tempJar.SetCookies(biliURL, cookies)

		// Verify identity using /x/web-interface/nav
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
		}, nil

	default:
		return &PollResult{
			State:   QRStateFailed,
			Message: fmt.Sprintf("扫码失败 (code=%d: %s)", code, reply.Data.Message),
		}, fmt.Errorf("bilibili qr login failed: code=%d", code)
	}
}

// verifyNav calls Bilibili nav endpoint to verify active session and fetch user profile.
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
