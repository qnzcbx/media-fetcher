package extractor

import "time"

// MediaType identifies the primary nature of the extracted media.
type MediaType string

const (
	MediaTypeVideo MediaType = "video"
	MediaTypeAudio MediaType = "audio"
	MediaTypeImage MediaType = "image"
)

// VideoStream describes an individual downloadable video stream track.
type VideoStream struct {
	ID           int      `json:"id"`            // Quality id (e.g. 116, 80, 64, 32, 16)
	QualityLabel string   `json:"quality_label"` // Readable quality (e.g. "1080P60", "1080P", "720P", "480P")
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	Codec        string   `json:"codec"` // e.g. "avc1.640028", "hev1.1.6.L120.90", "av01.0.08M.10"
	Bandwidth    int64    `json:"bandwidth"`
	URL          string   `json:"url"`
	BackupURLs   []string `json:"backup_urls,omitempty"`
	NeedAuth     bool     `json:"need_auth"`
}

// AudioStream describes an individual downloadable audio stream track.
type AudioStream struct {
	ID         int      `json:"id"` // e.g. 30280 (192K), 30232 (132K), 30216 (64K)
	Quality    string   `json:"quality"`
	Codec      string   `json:"codec"`
	Bandwidth  int64    `json:"bandwidth"`
	URL        string   `json:"url"`
	BackupURLs []string `json:"backup_urls,omitempty"`
}

// ImageItem describes a standalone image asset (e.g. for Lofter posts).
type ImageItem struct {
	Index    int    `json:"index"`
	URL      string `json:"url"`
	Filename string `json:"filename,omitempty"`
}

// MediaInfo contains unified, structured information about extracted media from any supported site.
type MediaInfo struct {
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

// ExtractOptions configures options for an extraction call.
type ExtractOptions struct {
	AccountID   string // Optional: account to use. If empty, anonymous mode is used.
	RequireAuth bool   // If true, fails with ErrCredentialExpired rather than falling back to anonymous
}
