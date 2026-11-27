package extractor

import "time"

// MediaType 标识解析出的媒体主要类型。
type MediaType string

const (
	MediaTypeVideo MediaType = "video"
	MediaTypeAudio MediaType = "audio"
	MediaTypeImage MediaType = "image"
)

// VideoStream 描述单个可下载的视频流轨道信息。
type VideoStream struct {
	FrameRate       string            `json:"frame_rate,omitempty"`
	DynamicRange    string            `json:"dynamic_range,omitempty"`
	MIMEType        string            `json:"mime_type,omitempty"`
	Size            int64             `json:"size,omitempty"`
	ID              int               `json:"id"`                          // 兼容既有整数画质/格式代码（例如 Bilibili 116/80 或 YouTube itag）
	QualityGroupKey string            `json:"quality_group_key,omitempty"` // 前端画质卡片分组键（如 "1080p-60fps-SDR"）；为空时回退按 ID 分组
	StreamKey       string            `json:"stream_key,omitempty"`        // 平台内唯一轨道键，用于精确选择与下载前重解析匹配
	Container       string            `json:"container,omitempty"`         // 源容器格式（如 "mp4"、"webm"）
	Headers         map[string]string `json:"headers,omitempty"`           // 下载该轨道所需的经过筛选的请求头
	ChunkSize       int64             `json:"chunk_size,omitempty"`        // 建议顺序 Range 分块大小（字节）；0 表示整文件下载
	QualityLabel    string            `json:"quality_label"`               // 可读画质标签（例如 "1080P60", "1080P", "720P", "480P"）
	Width           int               `json:"width"`
	Height          int               `json:"height"`
	Codec           string            `json:"codec"` // 编码格式（例如 "avc1.640028", "hev1.1.6.L120.90", "av01.0.08M.10", "vp9"）
	Bandwidth       int64             `json:"bandwidth"`
	URL             string            `json:"url"`
	BackupURLs      []string          `json:"backup_urls,omitempty"`
	NeedAuth        bool              `json:"need_auth"`
}

// AudioStream 描述单个可下载的音频流轨道信息。
type AudioStream struct {
	Kind       string            `json:"kind,omitempty"`
	MIMEType   string            `json:"mime_type,omitempty"`
	Size       int64             `json:"size,omitempty"`
	ID         int               `json:"id"`                   // 兼容既有整数音质/格式代码（例如 30280 (192K), 30232 (132K)）
	StreamKey  string            `json:"stream_key,omitempty"` // 平台内唯一轨道键（如 "140"、"140-drc"、"251"）
	Container  string            `json:"container,omitempty"`  // 源容器格式（如 "m4a"、"webm"）
	Headers    map[string]string `json:"headers,omitempty"`    // 下载该轨道所需的经过筛选的请求头
	ChunkSize  int64             `json:"chunk_size,omitempty"` // 建议顺序 Range 分块大小（字节）；0 表示整文件下载
	Quality    string            `json:"quality"`
	Codec      string            `json:"codec"`
	Bandwidth  int64             `json:"bandwidth"`
	URL        string            `json:"url"`
	BackupURLs []string          `json:"backup_urls,omitempty"`
}

// ImageItem 描述独立静态图片资源（例如 Lofter 图片博文图片项）。
type ImageItem struct {
	Index      int      `json:"index"`
	URL        string   `json:"url"`
	BackupURLs []string `json:"backup_urls,omitempty"`
	Filename   string   `json:"filename,omitempty"`
}

// Episode 是可以选择的单集，下载时按 URL 重新解析。
type Episode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// MediaInfo 统一且结构化地承载自各站点解析出的媒体资源元数据与流描述符。
type MediaInfo struct {
	Episodes      []Episode     `json:"episodes,omitempty"`
	SeasonTitle   string        `json:"season_title,omitempty"`
	EpisodeTitle  string        `json:"episode_title,omitempty"`
	SiteID        string        `json:"site_id"`
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	Author        string        `json:"author,omitempty"`
	CoverURL      string        `json:"cover_url,omitempty"`
	Duration      time.Duration `json:"duration,omitempty"`
	Type          MediaType     `json:"type"`
	IsAnonymous   bool          `json:"is_anonymous"`
	UsedAccountID string        `json:"used_account_id,omitempty"`
	VideoStreams  []VideoStream `json:"video_streams,omitempty"`
	AudioStreams  []AudioStream `json:"audio_streams,omitempty"`
	Images        []ImageItem   `json:"images,omitempty"`
	SourceURL     string        `json:"source_url"`
}

// ExtractOptions 配置解析调用的参数与认证要求。
type ExtractOptions struct {
	AccountID   string // 可选：指定要使用的站点账号 ID。若为空则使用匿名模式。
	RequireAuth bool   // 若为 true，凭据不可用时报错 ErrCredentialExpired，严禁静默回退至匿名低画质。
}
