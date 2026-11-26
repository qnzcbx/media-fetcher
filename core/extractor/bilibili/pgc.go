package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"media-fetcher/core/extractor"
)

var pgcRE = regexp.MustCompile(`^/bangumi/play/(ep|ss)([1-9][0-9]*)/?$`)

func parsePGCTarget(raw string) (string, string) {
	if link := linkRE.FindString(raw); link != "" {
		raw = strings.TrimRight(link, `，。),。）》】`)
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return "", ""
	}
	switch strings.ToLower(u.Hostname()) {
	case "bilibili.com", "www.bilibili.com", "m.bilibili.com":
	default:
		return "", ""
	}
	m := pgcRE.FindStringSubmatch(u.Path)
	if m == nil {
		return "", ""
	}
	return m[1], m[2]
}

type pgcEpisode struct {
	ID        int64  `json:"id"`
	CID       int64  `json:"cid"`
	Title     string `json:"title"`
	LongTitle string `json:"long_title"`
	Cover     string `json:"cover"`
	Duration  int64  `json:"duration"`
}

func episodeURL(id int64) string {
	return fmt.Sprintf("https://www.bilibili.com/bangumi/play/ep%d/", id)
}

func (e *BilibiliExtractor) extractPGC(ctx context.Context, client *http.Client, kind, id, account string, anon bool) (*extractor.MediaInfo, error) {
	referer := "https://www.bilibili.com/bangumi/play/" + kind + id + "/"
	key := "ep_id"
	if kind == "ss" {
		key = "season_id"
	}
	body, err := httpGet(ctx, client, strings.TrimRight(e.apiBase, "/")+"/pgc/view/web/season?"+key+"="+id, referer)
	if err != nil {
		return nil, fmt.Errorf("%w: 获取番剧信息: %v", extractor.ErrNetworkFailed, err)
	}
	var season struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Result  struct {
			Title    string       `json:"title"`
			Cover    string       `json:"cover"`
			Episodes []pgcEpisode `json:"episodes"`
			Section  []struct {
				Episodes []pgcEpisode `json:"episodes"`
			} `json:"section"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &season); err != nil {
		return nil, fmt.Errorf("%w: 番剧元数据格式无效", extractor.ErrAPIDenied)
	}
	if season.Code != 0 {
		return nil, pgcAPIError(season.Code, season.Message)
	}
	episodes := season.Result.Episodes
	for _, section := range season.Result.Section {
		episodes = append(episodes, section.Episodes...)
	}
	media := &extractor.MediaInfo{SiteID: "bilibili", ID: kind + id, Title: season.Result.Title, SeasonTitle: season.Result.Title, CoverURL: season.Result.Cover, Type: extractor.MediaTypeVideo, IsAnonymous: anon, UsedAccountID: account, SourceURL: referer}
	var selected *pgcEpisode
	seen := map[int64]bool{}
	for i := range episodes {
		ep := &episodes[i]
		if ep.ID <= 0 || ep.CID <= 0 || seen[ep.ID] {
			continue
		}
		seen[ep.ID] = true
		title := strings.TrimSpace(ep.Title + " " + ep.LongTitle)
		media.Episodes = append(media.Episodes, extractor.Episode{ID: strconv.FormatInt(ep.ID, 10), Title: title, URL: episodeURL(ep.ID)})
		if kind == "ep" && strconv.FormatInt(ep.ID, 10) == id {
			selected = ep
		}
	}
	if kind == "ss" {
		if len(media.Episodes) == 0 {
			return nil, fmt.Errorf("%w: 没有可选剧集", extractor.ErrUnsupportedMedia)
		}
		return media, nil
	}
	if selected == nil {
		return nil, fmt.Errorf("%w: 季度信息中没有指定剧集", extractor.ErrAPIDenied)
	}
	media.EpisodeTitle = strings.TrimSpace(selected.Title + " " + selected.LongTitle)
	media.Title = strings.TrimSpace(media.SeasonTitle + " " + media.EpisodeTitle)
	media.SourceURL = episodeURL(selected.ID)
	media.Duration = time.Duration(selected.Duration) * time.Millisecond
	if selected.Cover != "" {
		media.CoverURL = selected.Cover
	}
	q := url.Values{"ep_id": {id}, "cid": {strconv.FormatInt(selected.CID, 10)}, "fnval": {"16"}, "fnver": {"0"}}
	body, err = httpGet(ctx, client, strings.TrimRight(e.apiBase, "/")+"/pgc/player/web/v2/playurl?"+q.Encode(), media.SourceURL)
	if err != nil {
		return nil, fmt.Errorf("%w: 获取番剧播放资源: %v", extractor.ErrNetworkFailed, err)
	}
	var play struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Result  struct {
			VideoInfo struct {
				Dash        rawDash `json:"dash"`
				Code        int     `json:"code"`
				Message     string  `json:"message"`
				IsPreview   int     `json:"is_preview"`
				IsDRM       bool    `json:"is_drm"`
				DRMTechType int     `json:"drm_tech_type"`
			} `json:"video_info"`
			PlayCheck struct {
				PlayDetail string `json:"play_detail"`
			} `json:"play_check"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &play); err != nil {
		return nil, fmt.Errorf("%w: 番剧播放响应格式无效", extractor.ErrAPIDenied)
	}
	if play.Code != 0 {
		return nil, pgcAPIError(play.Code, play.Message)
	}
	v := play.Result.VideoInfo
	if v.IsDRM || v.DRMTechType != 0 {
		return nil, fmt.Errorf("%w: DRM 资源", extractor.ErrUnsupportedMedia)
	}
	if v.Code != 0 {
		return nil, pgcAPIError(v.Code, v.Message)
	}
	if v.IsPreview != 0 {
		return nil, fmt.Errorf("%w: 当前仅能试看，请检查登录或会员权限", extractor.ErrAPIDenied)
	}
	switch play.Result.PlayCheck.PlayDetail {
	case "PLAY_PREVIEW", "preview":
		return nil, fmt.Errorf("%w: 当前仅能试看，请检查登录或会员权限", extractor.ErrAPIDenied)
	case "PLAY_NONE", "none":
		return nil, fmt.Errorf("%w: 当前剧集不可播放", extractor.ErrAPIDenied)
	}
	if len(v.Dash.Video) == 0 {
		return nil, fmt.Errorf("%w: 未返回可用 DASH 视频（可能受权限或地区限制）", extractor.ErrUnsupportedMedia)
	}
	appendDASH(media, v.Dash)
	return media, nil
}

func pgcAPIError(code int, message string) error {
	if code == -101 {
		return fmt.Errorf("%w: 番剧需要登录 (%d): %s", extractor.ErrCredentialExpired, code, message)
	}
	return fmt.Errorf("%w: 番剧接口 (%d): %s", extractor.ErrAPIDenied, code, message)
}
