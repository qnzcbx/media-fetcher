package bilibili

import (
	"encoding/json"
	"math"
	"net/url"
	"strconv"
	"strings"

	"media-fetcher/core/extractor"
)

const (
	capabilityDASH        = 16
	capabilityHDR         = 64
	capability4K          = 128
	capabilityDolbyAudio  = 256
	capabilityDolbyVision = 512
	capability8K          = 1024
	capabilityAV1         = 2048
	webMediaCapabilities  = capabilityDASH | capabilityHDR | capability4K | capabilityDolbyAudio | capabilityDolbyVision | capability8K | capabilityAV1
)

type supportFormat struct {
	Quality     int    `json:"quality"`
	Description string `json:"new_description"`
	Display     string `json:"display_desc"`
}

func streamURL(s rawStream) string {
	if s.BaseURL != "" {
		return s.BaseURL
	}
	return s.BaseURL2
}
func validStreamURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Host != "" && (u.Scheme == "https" || u.Scheme == "http")
}
func streamMIME(s rawStream) string {
	if s.MIME != "" {
		return s.MIME
	}
	return s.MIME2
}
func streamBackups(s rawStream) []string {
	var result []string
	for _, u := range append(s.BackupURL, s.Backup2...) {
		if validStreamURL(u) {
			result = append(result, u)
		}
	}
	return result
}
func streamFPS(s rawStream) string {
	raw := s.FPS
	if len(raw) == 0 || string(raw) == "null" {
		raw = s.FPS2
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		value = string(raw)
	}
	parts := strings.Split(value, "/")
	if len(parts) > 2 {
		return ""
	}
	for _, part := range parts {
		n, e := strconv.ParseFloat(part, 64)
		if e != nil || n <= 0 || math.IsNaN(n) || math.IsInf(n, 0) {
			return ""
		}
	}
	return value
}

func appendDASH(media *extractor.MediaInfo, dash rawDash, formats []supportFormat) {
	seen := map[string]bool{}
	for _, v := range dash.Video {
		u := streamURL(v)
		if !validStreamURL(u) {
			continue
		}
		key := strconv.Itoa(v.ID) + ":" + v.Codecs
		if seen[key] {
			continue
		}
		seen[key] = true
		label := qualityName(v.ID)
		for _, f := range formats {
			if f.Quality == v.ID {
				if f.Description != "" {
					label = f.Description
				} else if f.Display != "" {
					label = f.Display
				}
			}
		}
		dynamic := ""
		if v.ID == 125 {
			dynamic = "HDR10"
		}
		if v.ID == 126 {
			dynamic = "Dolby Vision"
		}
		media.VideoStreams = append(media.VideoStreams, extractor.VideoStream{
			ID: v.ID, QualityLabel: label, Width: v.Width, Height: v.Height, Codec: v.Codecs,
			Bandwidth: v.Bandwidth, URL: u, BackupURLs: streamBackups(v), MIMEType: streamMIME(v),
			FrameRate: streamFPS(v), DynamicRange: dynamic, Size: v.Size,
			Headers: map[string]string{"Referer": "https://www.bilibili.com"},
		})
	}
	appendAudio := func(streams []rawStream, kind string) {
		for _, a := range streams {
			u := streamURL(a)
			if !validStreamURL(u) {
				continue
			}
			// 按 ID、原始编码及音轨类型去重，保留不同规格供精确选择。
			key := strconv.Itoa(a.ID) + ":" + a.Codecs + ":" + kind
			if seen[key] {
				continue
			}
			seen[key] = true
			label := audioQualityName(a.ID)
			if kind == "hires" {
				label = "Hi-Res 无损"
			}
			if kind == "dolby" {
				label = "杜比音频"
			}
			media.AudioStreams = append(media.AudioStreams, extractor.AudioStream{
				ID: a.ID, Codec: a.Codecs, Kind: kind, Quality: label,
				Bandwidth: a.Bandwidth, URL: u, BackupURLs: streamBackups(a),
				MIMEType: streamMIME(a), Size: a.Size,
				Headers: map[string]string{"Referer": "https://www.bilibili.com"},
			})
		}
	}
	appendAudio(dash.Audio, "standard")
	appendAudio(dash.Dolby.Audio, "dolby")
	if dash.FLAC.Audio != nil {
		appendAudio([]rawStream{*dash.FLAC.Audio}, "hires")
	}
}
