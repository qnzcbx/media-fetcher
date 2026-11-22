package lofter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
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
	postPathRE = regexp.MustCompile(`^/post/([0-9a-fA-F]+)_([0-9a-fA-F]+)/?$`)
	imgTagRE   = regexp.MustCompile(`(?is)<img\b[^>]*?\b(?:src|data-src)\s*=\s*["']([^"']+)["']`)
)

// LofterExtractor 负责解析网易 LOFTER 博客文章中的图片资源。
type LofterExtractor struct {
	credService *auth.CredentialService
	apiURL      string
}

// New 创建新的 LofterExtractor 实例。
func New(credService *auth.CredentialService) *LofterExtractor {
	return &LofterExtractor{
		credService: credService,
		apiURL:      "https://api.lofter.com/oldapi/post/detail.api?product=lofter-android-7.9.10",
	}
}

// SetAPIURL 允许在单元测试中覆盖目标端点以重定向到本地 Mock 服务。
func (e *LofterExtractor) SetAPIURL(u string) {
	e.apiURL = u
}

// SiteID 返回平台标识 "lofter"。
func (e *LofterExtractor) SiteID() string {
	return "lofter"
}

// Match 检查传入 URL 是否指向 LOFTER 博文。
func (e *LofterExtractor) Match(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(host, ".lofter.com") {
		return false
	}
	return postPathRE.MatchString(u.Path)
}

type postTarget struct {
	BlogID string
	PostID string
}

func parseLofterIDs(rawURL string) (postTarget, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return postTarget{}, fmt.Errorf("%w: %v", extractor.ErrInvalidURL, err)
	}
	host := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(host, ".lofter.com") {
		return postTarget{}, fmt.Errorf("%w: not a lofter.com domain", extractor.ErrInvalidURL)
	}
	m := postPathRE.FindStringSubmatch(u.Path)
	if m == nil {
		return postTarget{}, fmt.Errorf("%w: missing /post/blog_post pattern", extractor.ErrInvalidURL)
	}
	blogUint, err := strconv.ParseUint(m[1], 16, 64)
	if err != nil {
		return postTarget{}, fmt.Errorf("%w: invalid blog id: %v", extractor.ErrInvalidURL, err)
	}
	postUint, err := strconv.ParseUint(m[2], 16, 64)
	if err != nil {
		return postTarget{}, fmt.Errorf("%w: invalid post id: %v", extractor.ErrInvalidURL, err)
	}
	return postTarget{
		BlogID: strconv.FormatUint(blogUint, 10),
		PostID: strconv.FormatUint(postUint, 10),
	}, nil
}

type apiResponse struct {
	Meta struct {
		Status int    `json:"status"`
		Msg    string `json:"msg"`
	} `json:"meta"`
	Response struct {
		Posts []json.RawMessage `json:"posts"`
	} `json:"response"`
}

type postDetails struct {
	Title      string          `json:"title"`
	Digest     string          `json:"digest"`
	Type       int             `json:"type"`
	PhotoLinks json.RawMessage `json:"photoLinks"`
	Images     json.RawMessage `json:"images"`
	Content    string          `json:"content"`
	BlogInfo   struct {
		BlogNickName string `json:"blogNickName"`
	} `json:"blogInfo"`
}

// Extract 获取 LOFTER 帖子详情并返回解析出的高清图片列表。
func (e *LofterExtractor) Extract(ctx context.Context, rawURL string, opt extractor.ExtractOptions) (*extractor.MediaInfo, error) {
	target, err := parseLofterIDs(rawURL)
	if err != nil {
		return nil, err
	}

	var client *http.Client
	isAnon := true
	if opt.AccountID != "" {
		if e.credService == nil {
			return nil, fmt.Errorf("%w: credential service not configured", extractor.ErrCredentialExpired)
		}
		c, err := e.credService.GetClient("lofter", opt.AccountID)
		if err != nil {
			return nil, fmt.Errorf("%w: cannot use requested account: %v", extractor.ErrCredentialExpired, err)
		}
		client = c
		isAnon = false
	} else {
		if e.credService != nil {
			client = e.credService.GetAnonymousClient("lofter")
		} else {
			client = &http.Client{Timeout: 25 * time.Second}
		}
	}

	form := url.Values{
		"targetblogid": {target.BlogID},
		"postid":       {target.PostID},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", extractor.ErrNetworkFailed, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", extractor.ErrNetworkFailed, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", extractor.ErrNetworkFailed, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP status %d", extractor.ErrAPIDenied, resp.StatusCode)
	}

	var res apiResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON response: %v", extractor.ErrAPIDenied, err)
	}
	if res.Meta.Status != 200 {
		return nil, fmt.Errorf("%w: LOFTER API error %d: %s", extractor.ErrAPIDenied, res.Meta.Status, res.Meta.Msg)
	}
	if len(res.Response.Posts) == 0 {
		return nil, fmt.Errorf("%w: LOFTER returned empty posts array", extractor.ErrAPIDenied)
	}

	rawPost := res.Response.Posts[0]
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(rawPost, &wrapper); err == nil {
		if nested, ok := wrapper["post"]; ok && len(nested) > 0 {
			rawPost = nested
		}
	}

	var details postDetails
	if err := json.Unmarshal(rawPost, &details); err != nil {
		return nil, fmt.Errorf("%w: unmarshal post details: %v", extractor.ErrAPIDenied, err)
	}

	imageURLs, err := extractImageURLs(details)
	if err != nil {
		return nil, err
	}
	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("%w: no extractable image resources found", extractor.ErrUnsupportedMedia)
	}

	title := details.Title
	if title == "" {
		title = details.Digest
	}
	if title == "" {
		title = fmt.Sprintf("LOFTER 帖子 %s", target.PostID)
	}

	media := &extractor.MediaInfo{
		SiteID:        "lofter",
		ID:            target.PostID,
		Title:         title,
		Author:        details.BlogInfo.BlogNickName,
		Type:          extractor.MediaTypeImage,
		IsAnonymous:   isAnon,
		UsedAccountID: opt.AccountID,
		SourceURL:     rawURL,
	}

	for i, u := range imageURLs {
		media.Images = append(media.Images, extractor.ImageItem{
			Index:    i + 1,
			URL:      u,
			Filename: fmt.Sprintf("%03d.jpg", i+1),
		})
	}

	if len(media.Images) > 0 {
		media.CoverURL = media.Images[0].URL
	}

	return media, nil
}

func extractImageURLs(p postDetails) ([]string, error) {
	switch p.Type {
	case 2: // 图片帖子
		return parsePhotoLinks(p.PhotoLinks)
	case 5: // 问答/图片帖子
		return parsePhotoLinks(p.Images)
	case 1: // 包含图片的图文长文章帖子
		var candidates []string
		for _, m := range imgTagRE.FindAllStringSubmatch(p.Content, -1) {
			candidates = append(candidates, html.UnescapeString(m[1]))
		}
		return cleanURLs(candidates), nil
	default:
		return nil, fmt.Errorf("%w: unsupported LOFTER post type %d", extractor.ErrUnsupportedMedia, p.Type)
	}
}

func parsePhotoLinks(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, errors.New("empty photoLinks")
	}
	if raw[0] == '"' {
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return nil, err
		}
		raw = []byte(str)
	}

	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	var urls []string
	for _, item := range items {
		// LOFTER API uses "orign"
		var original string
		if val, ok := item["orign"]; ok {
			_ = json.Unmarshal(val, &original)
		}
		if original != "" {
			urls = append(urls, original)
		}
	}
	return cleanURLs(urls), nil
}

func cleanURLs(candidates []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, raw := range candidates {
		trimmed := strings.TrimSpace(raw)
		u, err := url.Parse(trimmed)
		if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") {
			continue
		}
		if !seen[trimmed] {
			seen[trimmed] = true
			result = append(result, trimmed)
		}
	}
	return result
}
