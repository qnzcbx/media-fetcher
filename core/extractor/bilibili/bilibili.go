package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"media-fetcher/core/auth"
	"media-fetcher/core/extractor"
)

var (
	bvRE             = regexp.MustCompile(`^BV[0-9A-Za-z]{10}$`)
	linkRE           = regexp.MustCompile(`https?://[^\s<>"']+`)
	playinfoPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?:window\s*\.\s*)?__playinfo__\s*=\s*`),
		regexp.MustCompile(`window\s*\[\s*["']__playinfo__["']\s*\]\s*=\s*`),
	}
)

type rawStream struct {
	ID        int      `json:"id"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	Codecs    string   `json:"codecs"`
	Bandwidth int64    `json:"bandwidth"`
	BaseURL   string   `json:"baseUrl"`
	BaseURL2  string   `json:"base_url"`
	BackupURL []string `json:"backupUrl"`
}

type rawDash struct {
	Video []rawStream `json:"video"`
	Audio []rawStream `json:"audio"`
}

type rawPlayInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Dash rawDash           `json:"dash"`
		Durl []json.RawMessage `json:"durl"`
	} `json:"data"`
}

type rawViewReply struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		BVID  string `json:"bvid"`
		CID   int64  `json:"cid"`
		Title string `json:"title"`
		Pic   string `json:"pic"`
		Owner struct {
			Name string `json:"name"`
		} `json:"owner"`
		Pages []struct {
			CID  int64 `json:"cid"`
			Page int   `json:"page"`
		} `json:"pages"`
	} `json:"data"`
}

// BilibiliExtractor 负责解析哔哩哔哩视频与音频流。
type BilibiliExtractor struct {
	credService *auth.CredentialService
	apiBase     string
	webBase     string
}

// New 创建新的 BilibiliExtractor 实例。
func New(credService *auth.CredentialService) *BilibiliExtractor {
	return &BilibiliExtractor{
		credService: credService,
		apiBase:     "https://api.bilibili.com",
		webBase:     "https://www.bilibili.com",
	}
}

// SetBaseURLs 允许在单元测试中覆盖目标端点以重定向到本地 Mock 服务。
func (e *BilibiliExtractor) SetBaseURLs(apiBase, webBase string) {
	e.apiBase = apiBase
	e.webBase = webBase
}

// SiteID 返回平台标识 "bilibili"。
func (e *BilibiliExtractor) SiteID() string {
	return "bilibili"
}

// Match 检查传入 URL 是否指向哔哩哔哩视频。
func (e *BilibiliExtractor) Match(rawURL string) bool {
	raw := strings.TrimSpace(rawURL)
	if m := linkRE.FindString(raw); m != "" {
		raw = strings.TrimRight(m, `，。),。）》】`)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host != "bilibili.com" && host != "www.bilibili.com" && host != "m.bilibili.com" {
		return false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	return (len(parts) >= 2 && parts[0] == "video" && bvRE.MatchString(parts[1])) || pgcRE.MatchString(u.Path)
}

// VideoTarget 保存解析得到的 BVID 和分 P 序号。
type VideoTarget struct {
	BVID      string
	Page      int
	TargetURL string
}

func parseVideoTarget(raw string) (VideoTarget, error) {
	raw = strings.TrimSpace(raw)
	if m := linkRE.FindString(raw); m != "" {
		raw = strings.TrimRight(m, `，。),。）》】`)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return VideoTarget{}, fmt.Errorf("%w: %v", extractor.ErrInvalidURL, err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "video" || !bvRE.MatchString(parts[1]) {
		return VideoTarget{}, fmt.Errorf("%w: missing or invalid BV id", extractor.ErrInvalidURL)
	}
	page := 1
	if p := u.Query().Get("p"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n >= 1 {
			page = n
		}
	}
	clean := fmt.Sprintf("https://www.bilibili.com/video/%s/", parts[1])
	if page > 1 {
		clean += fmt.Sprintf("?p=%d", page)
	}
	return VideoTarget{
		BVID:      parts[1],
		Page:      page,
		TargetURL: clean,
	}, nil
}

// Extract 严格依据权限规则解析哔哩哔哩视频音轨流。
func (e *BilibiliExtractor) Extract(ctx context.Context, rawURL string, opt extractor.ExtractOptions) (*extractor.MediaInfo, error) {
	var client *http.Client
	isAnon := true

	if opt.AccountID != "" {
		if e.credService == nil {
			return nil, fmt.Errorf("%w: credential service not configured", extractor.ErrCredentialExpired)
		}
		c, err := e.credService.GetClient("bilibili", opt.AccountID)
		if err != nil {
			// 明确请求了指定账号但不可用或已过期 -> 严禁静默降级！
			return nil, fmt.Errorf("%w: cannot use requested account %s: %v", extractor.ErrCredentialExpired, opt.AccountID, err)
		}
		client = c
		isAnon = false
	} else {
		if opt.RequireAuth {
			return nil, fmt.Errorf("%w: authentication required but no account selected", extractor.ErrCredentialExpired)
		}
		if e.credService != nil {
			client = e.credService.GetAnonymousClient("bilibili")
		} else {
			client = &http.Client{Timeout: 25 * time.Second}
		}
	}

	if kind, id := parsePGCTarget(rawURL); kind != "" {
		return e.extractPGC(ctx, client, kind, id, opt.AccountID, isAnon)
	}
	target, err := parseVideoTarget(rawURL)
	if err != nil {
		return nil, err
	}

	// 1. 获取视频基本信息（标题、CID、UP主、封面）
	viewURL := fmt.Sprintf("%s/x/web-interface/view?bvid=%s", strings.TrimRight(e.apiBase, "/"), target.BVID)
	viewData, err := httpGet(ctx, client, viewURL, target.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to fetch video view info: %v", extractor.ErrNetworkFailed, err)
	}

	var vReply rawViewReply
	if err := json.Unmarshal(viewData, &vReply); err != nil {
		return nil, fmt.Errorf("%w: parse view reply: %v", extractor.ErrAPIDenied, err)
	}
	if vReply.Code != 0 {
		return nil, fmt.Errorf("%w: view API error %d: %s", extractor.ErrAPIDenied, vReply.Code, vReply.Message)
	}

	var cid int64
	if target.Page > len(vReply.Data.Pages) && len(vReply.Data.Pages) > 0 {
		return nil, fmt.Errorf("%w: requested page %d exceeds total %d pages", extractor.ErrInvalidURL, target.Page, len(vReply.Data.Pages))
	}
	if len(vReply.Data.Pages) > 0 {
		cid = vReply.Data.Pages[target.Page-1].CID
	} else {
		cid = vReply.Data.CID
	}
	if cid == 0 {
		return nil, fmt.Errorf("%w: cannot resolve CID for video", extractor.ErrAPIDenied)
	}

	// 2. 通过 WBI 接口获取 PlayInfo
	playInfo, err := e.fetchWbiPlayInfo(ctx, client, target.BVID, cid, target.TargetURL)
	if err != nil {
		return nil, err
	}

	// 3. 将流信息映射为统一的 MediaInfo
	media := &extractor.MediaInfo{
		SiteID:        "bilibili",
		ID:            target.BVID,
		Title:         vReply.Data.Title,
		Author:        vReply.Data.Owner.Name,
		CoverURL:      vReply.Data.Pic,
		Type:          extractor.MediaTypeVideo,
		IsAnonymous:   isAnon,
		UsedAccountID: opt.AccountID,
		SourceURL:     target.TargetURL,
	}

	appendDASH(media, playInfo.Data.Dash)
	return media, nil
}

func appendDASH(media *extractor.MediaInfo, dash rawDash) {
	for _, v := range dash.Video {
		u := v.BaseURL
		if u == "" {
			u = v.BaseURL2
		}
		media.VideoStreams = append(media.VideoStreams, extractor.VideoStream{
			ID:           v.ID,
			QualityLabel: qualityName(v.ID),
			Width:        v.Width,
			Height:       v.Height,
			Codec:        v.Codecs,
			Bandwidth:    v.Bandwidth,
			URL:          u,
			BackupURLs:   v.BackupURL,
			NeedAuth:     v.ID > 64, // 1080P 及以上需要登录或大会员
		})
	}

	for _, a := range dash.Audio {
		u := a.BaseURL
		if u == "" {
			u = a.BaseURL2
		}
		media.AudioStreams = append(media.AudioStreams, extractor.AudioStream{
			ID:         a.ID,
			Quality:    audioQualityName(a.ID),
			Codec:      a.Codecs,
			Bandwidth:  a.Bandwidth,
			URL:        u,
			BackupURLs: a.BackupURL,
		})
	}
}

func (e *BilibiliExtractor) fetchWbiPlayInfo(ctx context.Context, client *http.Client, bvid string, cid int64, referer string) (*rawPlayInfo, error) {
	// 1. 请求 nav 获取 WBI 的 img_url 和 sub_url
	navURL := fmt.Sprintf("%s/x/web-interface/nav", strings.TrimRight(e.apiBase, "/"))
	navData, err := httpGet(ctx, client, navURL, referer)
	if err != nil {
		return nil, fmt.Errorf("%w: fetch WBI nav: %v", extractor.ErrNetworkFailed, err)
	}

	var navReply struct {
		Data struct {
			WBI struct {
				Img string `json:"img_url"`
				Sub string `json:"sub_url"`
			} `json:"wbi_img"`
		} `json:"data"`
	}
	if err := json.Unmarshal(navData, &navReply); err != nil {
		return nil, fmt.Errorf("%w: parse WBI nav: %v", extractor.ErrAPIDenied, err)
	}

	imgKey := KeyFromImageURL(navReply.Data.WBI.Img)
	subKey := KeyFromImageURL(navReply.Data.WBI.Sub)
	mixinKey, err := MixinKey(imgKey, subKey)
	if err != nil {
		return nil, fmt.Errorf("%w: mixin WBI key: %v", extractor.ErrAPIDenied, err)
	}

	// 2. 构造 WBI 签名的 playurl 请求（fnval=16 请求 DASH 格式）
	q := url.Values{
		"bvid":  {bvid},
		"cid":   {strconv.FormatInt(cid, 10)},
		"fnval": {"16"},
		"fnver": {"0"},
	}
	signedQuery := SignQuery(q, mixinKey, time.Now())
	playURL := fmt.Sprintf("%s/x/player/wbi/playurl?%s", strings.TrimRight(e.apiBase, "/"), signedQuery)

	body, err := httpGet(ctx, client, playURL, referer)
	if err != nil {
		return nil, fmt.Errorf("%w: fetch playurl: %v", extractor.ErrNetworkFailed, err)
	}

	var info rawPlayInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("%w: parse playinfo JSON: %v", extractor.ErrAPIDenied, err)
	}
	if info.Code != 0 {
		return nil, fmt.Errorf("%w: playurl API returned code %d: %s", extractor.ErrAPIDenied, info.Code, info.Message)
	}
	if len(info.Data.Dash.Video) == 0 {
		return nil, fmt.Errorf("%w: playurl returned zero video streams", extractor.ErrUnsupportedMedia)
	}

	return &info, nil
}

func qualityName(id int) string {
	switch id {
	case 127:
		return "8K 超高清"
	case 120:
		return "4K 超清"
	case 116:
		return "1080P 60帧"
	case 80:
		return "1080P 高清"
	case 64:
		return "720P 高清"
	case 32:
		return "480P 清晰"
	case 16:
		return "360P 流畅"
	default:
		return fmt.Sprintf("画质 %d", id)
	}
}

func audioQualityName(id int) string {
	switch id {
	case 30280:
		return "192Kbps 高音质"
	case 30232:
		return "132Kbps 较高音质"
	case 30216:
		return "64Kbps 普通音质"
	default:
		return fmt.Sprintf("音轨 %d", id)
	}
}

func httpGet(ctx context.Context, client *http.Client, endpoint, referer string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", referer)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
}
