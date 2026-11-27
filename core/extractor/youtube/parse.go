package youtube

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"media-fetcher/core/downloader"
	"media-fetcher/core/extractor"
)

const defaultYouTubeChunkSize int64 = 10 << 20 // 10 MiB，与 yt-dlp 默认分块策略一致

type ytdlpInfo struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Uploader   string           `json:"uploader"`
	Channel    string           `json:"channel"`
	Duration   float64          `json:"duration"`
	Thumbnail  string           `json:"thumbnail"`
	Thumbnails []ytdlpThumbnail `json:"thumbnails"`
	IsLive     bool             `json:"is_live"`
	LiveStatus string           `json:"live_status"`
	WebpageURL string           `json:"webpage_url"`
	Formats    []ytdlpFormat    `json:"formats"`
}

type ytdlpThumbnail struct {
	URL        string `json:"url"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Preference int    `json:"preference"`
}

type ytdlpFormat struct {
	FormatID          string            `json:"format_id"`
	FormatNote        string            `json:"format_note"`
	Ext               string            `json:"ext"`
	Protocol          string            `json:"protocol"`
	URL               string            `json:"url"`
	Width             int               `json:"width"`
	Height            int               `json:"height"`
	FPS               float64           `json:"fps"`
	VCodec            string            `json:"vcodec"`
	ACodec            string            `json:"acodec"`
	DynamicRange      string            `json:"dynamic_range"`
	TBR               float64           `json:"tbr"`
	VBR               float64           `json:"vbr"`
	ABR               float64           `json:"abr"`
	ASR               int               `json:"asr"`
	AudioChannels     int               `json:"audio_channels"`
	Filesize          *int64            `json:"filesize"`
	FilesizeApprox    *int64            `json:"filesize_approx"`
	Language          string            `json:"language"`
	LanguagePref      int               `json:"language_preference"`
	HasDRM            any               `json:"has_drm"`
	HTTPHeaders       map[string]string `json:"http_headers"`
	DownloaderOptions struct {
		HTTPChunkSize int64 `json:"http_chunk_size"`
	} `json:"downloader_options"`
}

// ParseYTDLPJSON 将 yt-dlp --dump-single-json 输出映射为统一的 MediaInfo。
func ParseYTDLPJSON(raw []byte, target Target) (*extractor.MediaInfo, error) {
	var info ytdlpInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("%w: 解析 yt-dlp JSON 失败: %v", extractor.ErrAPIDenied, err)
	}

	if info.IsLive || info.LiveStatus == "is_live" || info.LiveStatus == "is_upcoming" {
		return nil, fmt.Errorf("%w: 暂不支持直播流或待开播内容", extractor.ErrUnsupportedMedia)
	}

	videoID := strings.TrimSpace(info.ID)
	if videoID == "" {
		videoID = target.VideoID
	}
	author := strings.TrimSpace(info.Channel)
	if author == "" {
		author = strings.TrimSpace(info.Uploader)
	}
	sourceURL := target.CanonicalURL
	if sourceURL == "" && videoID != "" {
		sourceURL = "https://www.youtube.com/watch?v=" + videoID
	}

	media := &extractor.MediaInfo{
		SiteID:      "youtube",
		ID:          videoID,
		Title:       strings.TrimSpace(info.Title),
		Author:      author,
		CoverURL:    selectCoverURL(info),
		Duration:    time.Duration(math.Round(info.Duration)) * time.Second,
		Type:        extractor.MediaTypeVideo,
		IsAnonymous: true,
		SourceURL:   sourceURL,
	}

	// 预先记录所有非 DRC 的有效 HTTPS 音频基础键，用于隐藏完全重复的 DRC 变体，避免前端混淆。
	nonDRCAudio := map[string]bool{}
	hasDefaultAudioLang := false
	for _, f := range info.Formats {
		if !isUsableHTTPSFormat(f) || !isAudioOnly(f) {
			continue
		}
		if isDefaultOrOriginalAudio(f) {
			hasDefaultAudioLang = true
		}
		if !isDRCFormat(f) {
			nonDRCAudio[audioBaseIdentity(f)] = true
		}
	}

	var videos []extractor.VideoStream
	var audios []extractor.AudioStream
	seenVideoKeys := map[string]bool{}
	seenAudioKeys := map[string]bool{}
	seenDRMOnly := false
	seenUnsupportedProtoOnly := false

	for idx, f := range info.Formats {
		if isDRM(f.HasDRM) {
			seenDRMOnly = true
			continue
		}
		if !isValidMediaURL(f.URL) {
			continue
		}
		if strings.ToLower(strings.TrimSpace(f.Protocol)) != "https" {
			if f.Protocol != "" && f.Protocol != "mhtml" {
				seenUnsupportedProtoOnly = true
			}
			continue
		}
		// 严格过滤 Progressive 混合轨，不将其错误暴露为独立单轨。
		if isVideoOnly(f) {
			if isAISuperResolution(f) {
				continue
			}
			normV := downloader.NormalizeVideoCodec(f.VCodec)
			if normV == "" || f.Width <= 0 || f.Height <= 0 {
				continue
			}
			streamKey := strings.TrimSpace(f.FormatID)
			if streamKey == "" || seenVideoKeys[streamKey] {
				continue
			}
			seenVideoKeys[streamKey] = true

			tier := qualityResolution(f.Width, f.Height)
			fpsBucket := normalizeFPSBucket(f.FPS)
			dr := normalizeDynamicRange(f.DynamicRange)
			groupKey := buildQualityGroupKey(tier, fpsBucket, dr)
			label := buildQualityLabel(tier, fpsBucket, dr)

			var exactSize int64
			if f.Filesize != nil && *f.Filesize > 0 {
				exactSize = *f.Filesize
			}
			chunkSize := f.DownloaderOptions.HTTPChunkSize
			if chunkSize <= 0 {
				chunkSize = defaultYouTubeChunkSize
			}
			bw := bitrateToBps(f.VBR, f.TBR)
			fpsStr := ""
			if f.FPS > 0 {
				fpsStr = strconv.Itoa(int(math.Round(f.FPS)))
			}

			videos = append(videos, extractor.VideoStream{
				ID:              parseFormatNumericID(streamKey, 10000+idx),
				QualityGroupKey: groupKey,
				StreamKey:       streamKey,
				Container:       strings.ToLower(strings.TrimSpace(f.Ext)),
				Headers:         downloader.SanitizeHeaders(f.HTTPHeaders),
				ChunkSize:       chunkSize,
				QualityLabel:    label,
				Width:           f.Width,
				Height:          f.Height,
				Codec:           strings.TrimSpace(f.VCodec),
				Bandwidth:       bw,
				URL:             f.URL,
				MIMEType:        buildVideoMIME(f.Ext, f.VCodec),
				FrameRate:       fpsStr,
				DynamicRange:    dr,
				Size:            exactSize,
			})
		} else if isAudioOnly(f) {
			normA := downloader.NormalizeAudioCodec(f.ACodec)
			if normA == "" {
				continue
			}
			if isDRCFormat(f) && nonDRCAudio[audioBaseIdentity(f)] {
				// 存在对应的非 DRC 原音轨时默认隐藏 DRC 重复变体，不造成错误去重。
				continue
			}
			if hasDefaultAudioLang && !isDefaultOrOriginalAudio(f) && f.Language != "" {
				// 多语配音视频默认仅暴露原声/默认语言音轨，避免用户误下配音轨。
				continue
			}
			streamKey := strings.TrimSpace(f.FormatID)
			if streamKey == "" || seenAudioKeys[streamKey] {
				continue
			}
			seenAudioKeys[streamKey] = true

			var exactSize int64
			if f.Filesize != nil && *f.Filesize > 0 {
				exactSize = *f.Filesize
			}
			chunkSize := f.DownloaderOptions.HTTPChunkSize
			if chunkSize <= 0 {
				chunkSize = defaultYouTubeChunkSize
			}
			bw := bitrateToBps(f.ABR, f.TBR)
			kind := classifyAudioKind(normA, isDRCFormat(f))

			audios = append(audios, extractor.AudioStream{
				ID:        parseFormatNumericID(streamKey, 20000+idx),
				StreamKey: streamKey,
				Container: strings.ToLower(strings.TrimSpace(f.Ext)),
				Headers:   downloader.SanitizeHeaders(f.HTTPHeaders),
				ChunkSize: chunkSize,
				Kind:      kind,
				Quality:   buildAudioLabel(normA, bw, f.AudioChannels, isDRCFormat(f)),
				Codec:     strings.TrimSpace(f.ACodec),
				Bandwidth: bw,
				URL:       f.URL,
				MIMEType:  buildAudioMIME(f.Ext, f.ACodec),
				Size:      exactSize,
			})
		}
	}

	if len(videos) == 0 && len(audios) == 0 {
		if seenDRMOnly {
			return nil, fmt.Errorf("%w: 当前视频受 DRM 保护，不支持下载", extractor.ErrUnsupportedMedia)
		}
		if seenUnsupportedProtoOnly {
			return nil, fmt.Errorf("%w: 当前视频仅提供暂不支持的流媒体协议（如 HLS/DASH/SABR）", extractor.ErrUnsupportedMedia)
		}
		return nil, fmt.Errorf("%w: 未找到可下载的独立 HTTPS 音视频轨道", extractor.ErrUnsupportedMedia)
	}

	sortVideoStreams(videos)
	videos = dedupeVideoStreamsByGroupAndCodec(videos)
	sortAudioStreams(audios)

	media.VideoStreams = videos
	media.AudioStreams = audios
	return media, nil
}

// dedupeVideoStreamsByGroupAndCodec 在同一画质分组与同一编码族内保留码率最高的一条轨道，
// 避免 Shorts 等视频中出现多个同分辨率同编码的码率变体导致前端编码切换器出现重复按钮。
func dedupeVideoStreamsByGroupAndCodec(sorted []extractor.VideoStream) []extractor.VideoStream {
	seen := make(map[string]bool, len(sorted))
	out := make([]extractor.VideoStream, 0, len(sorted))
	for _, s := range sorted {
		key := s.QualityGroupKey + "|" + downloader.NormalizeVideoCodec(s.Codec)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

func isUsableHTTPSFormat(f ytdlpFormat) bool {
	return !isDRM(f.HasDRM) && strings.ToLower(strings.TrimSpace(f.Protocol)) == "https" && isValidMediaURL(f.URL)
}

func isVideoOnly(f ytdlpFormat) bool {
	v := strings.ToLower(strings.TrimSpace(f.VCodec))
	a := strings.ToLower(strings.TrimSpace(f.ACodec))
	return v != "" && v != "none" && (a == "" || a == "none")
}

func isAudioOnly(f ytdlpFormat) bool {
	v := strings.ToLower(strings.TrimSpace(f.VCodec))
	a := strings.ToLower(strings.TrimSpace(f.ACodec))
	return a != "" && a != "none" && (v == "" || v == "none")
}

func isDRM(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "maybe" || s == "yes"
	default:
		return false
	}
}

func isAISuperResolution(f ytdlpFormat) bool {
	return strings.HasSuffix(strings.ToLower(f.FormatID), "-sr") ||
		strings.Contains(strings.ToLower(f.FormatNote), "ai-upscaled")
}

func isDRCFormat(f ytdlpFormat) bool {
	return strings.HasSuffix(strings.ToLower(f.FormatID), "-drc") ||
		strings.Contains(strings.ToUpper(f.FormatNote), "DRC")
}

func audioBaseIdentity(f ytdlpFormat) string {
	id := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(f.FormatID)), "-drc")
	return id + "|" + strings.ToLower(strings.TrimSpace(f.Language))
}

func isDefaultOrOriginalAudio(f ytdlpFormat) bool {
	note := strings.ToLower(f.FormatNote)
	return f.LanguagePref > 0 || strings.Contains(note, "(default)") || strings.Contains(note, "(original)")
}

func isValidMediaURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Hostname() != "" && (u.Scheme == "https" || u.Scheme == "http")
}

// qualityResolution 使用短边决定画质档位，确保横屏 1920x1080 与竖屏 Shorts 1080x1920 均映射为 1080P。
func qualityResolution(w, h int) int {
	short := min(w, h)
	switch {
	case short >= 4320:
		return 4320
	case short >= 2160:
		return 2160
	case short >= 1440:
		return 1440
	case short >= 1080:
		return 1080
	case short >= 720:
		return 720
	case short >= 480:
		return 480
	case short >= 360:
		return 360
	case short >= 240:
		return 240
	default:
		return short
	}
}

func normalizeFPSBucket(fps float64) int {
	switch {
	case fps >= 55:
		return 60
	case fps >= 45:
		return 50
	default:
		return 30
	}
}

func normalizeDynamicRange(dr string) string {
	s := strings.TrimSpace(dr)
	if s == "" || strings.EqualFold(s, "SDR") {
		return ""
	}
	return s
}

func buildQualityGroupKey(tier, fpsBucket int, dr string) string {
	drTag := "SDR"
	if dr != "" {
		drTag = strings.ToUpper(strings.ReplaceAll(dr, " ", ""))
	}
	return fmt.Sprintf("%dp-%dfps-%s", tier, fpsBucket, drTag)
}

func buildQualityLabel(tier, fpsBucket int, dr string) string {
	var base string
	switch tier {
	case 4320:
		base = "8K"
	case 2160:
		base = "4K"
	case 1440:
		base = "2K"
	default:
		base = fmt.Sprintf("%dP", tier)
	}
	if fpsBucket >= 50 {
		base += fmt.Sprintf(" %d帧", fpsBucket)
	}
	if dr != "" {
		base += " " + dr
	}
	return base
}

func parseFormatNumericID(formatID string, fallback int) int {
	prefix := formatID
	if i := strings.IndexByte(prefix, '-'); i > 0 {
		prefix = prefix[:i]
	}
	if n, err := strconv.Atoi(prefix); err == nil && n > 0 {
		return n
	}
	return fallback
}

func bitrateToBps(primaryKbps, fallbackKbps float64) int64 {
	kbps := primaryKbps
	if kbps <= 0 {
		kbps = fallbackKbps
	}
	if kbps <= 0 {
		return 0
	}
	return int64(math.Round(kbps * 1000))
}

func classifyAudioKind(normAudio string, isDRC bool) string {
	switch normAudio {
	case "ac3", "eac3":
		return "dolby"
	case "flac":
		return "hires"
	default:
		if isDRC {
			return "drc"
		}
		return "standard"
	}
}

func buildAudioLabel(normAudio string, bps int64, channels int, isDRC bool) string {
	kbps := int(math.Round(float64(bps) / 1000.0))
	codecName := strings.ToUpper(normAudio)
	switch normAudio {
	case "aac":
		codecName = "AAC"
	case "opus":
		codecName = "Opus"
	case "eac3":
		codecName = "Dolby E-AC-3"
	case "ac3":
		codecName = "Dolby AC-3"
	case "vorbis":
		codecName = "Vorbis"
	}
	parts := []string{}
	if kbps > 0 {
		parts = append(parts, fmt.Sprintf("%dKbps", kbps))
	}
	if channels >= 6 {
		parts = append(parts, "5.1")
	}
	parts = append(parts, codecName)
	if isDRC {
		parts = append(parts, "DRC")
	}
	return strings.Join(parts, " ")
}

func buildVideoMIME(ext, codec string) string {
	sub := strings.ToLower(strings.TrimSpace(ext))
	if sub == "" {
		sub = "mp4"
	}
	return fmt.Sprintf("video/%s; codecs=%q", sub, strings.TrimSpace(codec))
}

func buildAudioMIME(ext, codec string) string {
	sub := strings.ToLower(strings.TrimSpace(ext))
	if sub == "m4a" || sub == "" {
		sub = "mp4"
	}
	return fmt.Sprintf("audio/%s; codecs=%q", sub, strings.TrimSpace(codec))
}

func selectCoverURL(info ytdlpInfo) string {
	// 优先选择清晰度适中（<= 1280px）且可直接解码的 JPG/WebP 封面，避免触发预览 2MiB 上限
	bestURL := ""
	bestScore := -1
	for _, t := range info.Thumbnails {
		if !isValidMediaURL(t.URL) {
			continue
		}
		score := t.Width * t.Height
		if strings.Contains(t.URL, "hqdefault") || strings.Contains(t.URL, "sddefault") || strings.Contains(t.URL, "maxresdefault") {
			score += 100000
		}
		if t.Width > 1920 {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestURL = t.URL
		}
	}
	if bestURL != "" {
		return bestURL
	}
	if isValidMediaURL(info.Thumbnail) {
		return info.Thumbnail
	}
	return ""
}

func videoCodecOrder(codec string) int {
	switch downloader.NormalizeVideoCodec(codec) {
	case "avc":
		return 0
	case "hevc":
		return 1
	case "vp9":
		return 2
	case "av1":
		return 3
	default:
		return 4
	}
}

func sortVideoStreams(streams []extractor.VideoStream) {
	sort.SliceStable(streams, func(i, j int) bool {
		a, b := streams[i], streams[j]
		ti, tj := qualityResolution(a.Width, a.Height), qualityResolution(b.Width, b.Height)
		if ti != tj {
			return ti > tj
		}
		fi, _ := strconv.Atoi(a.FrameRate)
		fj, _ := strconv.Atoi(b.FrameRate)
		bi, bj := normalizeFPSBucket(float64(fi)), normalizeFPSBucket(float64(fj))
		if bi != bj {
			return bi > bj
		}
		if (a.DynamicRange != "") != (b.DynamicRange != "") {
			return a.DynamicRange == "" // SDR 默认排在同档位前，兼容性更佳
		}
		ci, cj := videoCodecOrder(a.Codec), videoCodecOrder(b.Codec)
		if ci != cj {
			return ci < cj
		}
		return a.Bandwidth > b.Bandwidth
	})
}

func sortAudioStreams(streams []extractor.AudioStream) {
	sort.SliceStable(streams, func(i, j int) bool {
		a, b := streams[i], streams[j]
		rank := func(s extractor.AudioStream) int {
			switch downloader.NormalizeAudioCodec(s.Codec) {
			case "aac":
				if s.Kind == "drc" {
					return 3
				}
				return 0
			case "opus":
				if s.Kind == "drc" {
					return 4
				}
				return 1
			case "eac3", "ac3":
				return 2
			default:
				return 5
			}
		}
		ri, rj := rank(a), rank(b)
		if ri != rj {
			return ri < rj
		}
		return a.Bandwidth > b.Bandwidth
	})
}
