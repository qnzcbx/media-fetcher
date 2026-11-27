package youtube

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"media-fetcher/core/extractor"
)

var (
	linkRE    = regexp.MustCompile(`https?://[^\s<>"'，。！？；【】（）]+`)
	videoIDRE = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
)

// Target 描述规范化后的单视频 YouTube 目标。
type Target struct {
	VideoID      string
	CanonicalURL string
	IsShorts     bool
}

// ParseTarget 严格校验协议、域名、路径与 11 位视频 ID，并剥离播放列表与追踪参数。
func ParseTarget(raw string) (Target, error) {
	s := strings.TrimSpace(raw)
	if m := linkRE.FindString(s); m != "" {
		s = strings.TrimRight(m, `，。！？；,.)）】》]`)
	}
	u, err := url.Parse(s)
	if err != nil {
		return Target{}, fmt.Errorf("%w: 无法解析 YouTube 链接: %v", extractor.ErrInvalidURL, err)
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return Target{}, fmt.Errorf("%w: 不支持的链接协议或包含非法用户信息", extractor.ErrInvalidURL)
	}

	host := strings.ToLower(u.Hostname())
	switch host {
	case "youtube.com", "www.youtube.com", "m.youtube.com":
		cleanPath := strings.Trim(u.Path, "/")
		if cleanPath == "watch" {
			vid := u.Query().Get("v")
			if !videoIDRE.MatchString(vid) {
				return Target{}, fmt.Errorf("%w: 缺少有效的 YouTube 视频 ID", extractor.ErrInvalidURL)
			}
			return Target{
				VideoID:      vid,
				CanonicalURL: "https://www.youtube.com/watch?v=" + vid,
			}, nil
		}
		parts := strings.Split(cleanPath, "/")
		if len(parts) == 2 && parts[0] == "shorts" && videoIDRE.MatchString(parts[1]) {
			return Target{
				VideoID:      parts[1],
				CanonicalURL: "https://www.youtube.com/shorts/" + parts[1],
				IsShorts:     true,
			}, nil
		}
		return Target{}, fmt.Errorf("%w: 首阶段仅支持 YouTube 单视频与 Shorts 链接", extractor.ErrInvalidURL)

	case "youtu.be":
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) == 1 && videoIDRE.MatchString(parts[0]) {
			return Target{
				VideoID:      parts[0],
				CanonicalURL: "https://www.youtube.com/watch?v=" + parts[0],
			}, nil
		}
		return Target{}, fmt.Errorf("%w: 无效的 youtu.be 短链接", extractor.ErrInvalidURL)

	default:
		return Target{}, fmt.Errorf("%w: 不支持的域名 %q", extractor.ErrInvalidURL, host)
	}
}
