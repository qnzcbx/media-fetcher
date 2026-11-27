package youtube

import (
	"context"
	"errors"
	"strings"
	"testing"

	"media-fetcher/core/extractor"
)

func TestParseTarget(t *testing.T) {
	valid := []struct {
		in       string
		wantID   string
		wantURL  string
		isShorts bool
	}{
		{"https://www.youtube.com/watch?v=aqz-KE-bpKQ", "aqz-KE-bpKQ", "https://www.youtube.com/watch?v=aqz-KE-bpKQ", false},
		{"https://youtube.com/watch?v=YE7VzlLtp-4&list=PL123&index=2&si=track", "YE7VzlLtp-4", "https://www.youtube.com/watch?v=YE7VzlLtp-4", false},
		{"https://m.youtube.com/watch?v=aqz-KE-bpKQ&t=12", "aqz-KE-bpKQ", "https://www.youtube.com/watch?v=aqz-KE-bpKQ", false},
		{"https://youtu.be/aqz-KE-bpKQ?si=secret", "aqz-KE-bpKQ", "https://www.youtube.com/watch?v=aqz-KE-bpKQ", false},
		{"https://www.youtube.com/shorts/aqz-KE-bpKQ?feature=share", "aqz-KE-bpKQ", "https://www.youtube.com/shorts/aqz-KE-bpKQ", true},
		{"分享视频 https://youtu.be/aqz-KE-bpKQ，快来看！", "aqz-KE-bpKQ", "https://www.youtube.com/watch?v=aqz-KE-bpKQ", false},
	}
	for _, tc := range valid {
		got, err := ParseTarget(tc.in)
		if err != nil {
			t.Errorf("ParseTarget(%q) unexpected err: %v", tc.in, err)
			continue
		}
		if got.VideoID != tc.wantID || got.CanonicalURL != tc.wantURL || got.IsShorts != tc.isShorts {
			t.Errorf("ParseTarget(%q) = %+v, want id=%s url=%s shorts=%v", tc.in, got, tc.wantID, tc.wantURL, tc.isShorts)
		}
	}

	invalid := []string{
		"https://youtube.com.evil.org/watch?v=aqz-KE-bpKQ",
		"https://fakeyoutu.be/aqz-KE-bpKQ",
		"https://user:pass@www.youtube.com/watch?v=aqz-KE-bpKQ",
		"javascript:alert(1)",
		"https://www.youtube.com/playlist?list=PL1234567890",
		"https://www.youtube.com/watch?v=short",
		"https://www.youtube.com/shorts/aqz-KE-bpKQ/extra",
		"https://www.bilibili.com/video/BV1xx411c7mD",
	}
	for _, raw := range invalid {
		if _, err := ParseTarget(raw); !errors.Is(err, extractor.ErrInvalidURL) {
			t.Errorf("ParseTarget(%q) expected ErrInvalidURL, got %v", raw, err)
		}
	}
}

func TestParseAndCheckJSVersion(t *testing.T) {
	cases := []struct {
		kind, out string
		wantOK    bool
	}{
		{"quickjs", "0.18.0\n", true},
		{"quickjs", "0.11.0\n", false},
		{"node", "v22.22.2\n", true},
		{"node", "v20.18.0\n", false},
		{"deno", "deno 2.9.8 (release)\n", true},
		{"deno", "deno 2.2.0\n", false},
		{"bun", "1.3.0\n", false},
	}
	for _, tc := range cases {
		_, ok := parseAndCheckJSVersion(tc.kind, tc.out)
		if ok != tc.wantOK {
			t.Errorf("(%s, %q) ok=%v want=%v", tc.kind, tc.out, ok, tc.wantOK)
		}
	}
}

func TestParseYTDLPJSON_GroupingFilteringAndHeaders(t *testing.T) {
	fixture := `{
		"id": "aqz-KE-bpKQ",
		"title": "Big Buck Bunny 60fps 4K",
		"channel": "Blender",
		"duration": 635,
		"is_live": false,
		"live_status": "not_live",
		"thumbnails": [
			{"url": "https://i.ytimg.com/vi/aqz-KE-bpKQ/hqdefault.jpg", "width": 480, "height": 360},
			{"url": "https://i.ytimg.com/vi/aqz-KE-bpKQ/maxresdefault.jpg", "width": 1280, "height": 720}
		],
		"formats": [
			{"format_id": "sb0", "protocol": "mhtml", "url": "https://i.ytimg.com/sb.mhtml", "vcodec": "none", "acodec": "none"},
			{"format_id": "628", "protocol": "m3u8_native", "url": "https://example.com/628.m3u8", "ext": "mp4", "width": 3840, "height": 2160, "fps": 60, "vcodec": "vp09.00.51.08", "acodec": "none"},
			{"format_id": "18", "protocol": "https", "url": "https://rr1.googlevideo.com/18", "ext": "mp4", "width": 640, "height": 360, "fps": 30, "vcodec": "avc1.42001E", "acodec": "mp4a.40.2"},
			{"format_id": "248-sr", "format_note": "1080p, AI-upscaled", "protocol": "https", "url": "https://rr1.googlevideo.com/248sr", "ext": "webm", "width": 1920, "height": 1080, "fps": 24, "vcodec": "vp9", "acodec": "none", "filesize": 96000000},
			{"format_id": "299", "protocol": "https", "url": "https://rr1.googlevideo.com/299", "ext": "mp4", "width": 1920, "height": 1080, "fps": 60, "vcodec": "avc1.64002A", "acodec": "none", "dynamic_range": "SDR", "tbr": 6900, "filesize": 257619653, "http_headers": {"User-Agent": "CustomUA/1.0", "Cookie": "bad=1"}, "downloader_options": {"http_chunk_size": 10485760}},
			{"format_id": "303", "protocol": "https", "url": "https://rr1.googlevideo.com/303", "ext": "webm", "width": 1920, "height": 1080, "fps": 60, "vcodec": "vp9", "acodec": "none", "dynamic_range": "SDR", "tbr": 4652, "filesize": 168736189, "http_headers": {"User-Agent": "CustomUA/1.0"}, "downloader_options": {"http_chunk_size": 10485760}},
			{"format_id": "335", "protocol": "https", "url": "https://rr1.googlevideo.com/335", "ext": "webm", "width": 1920, "height": 1080, "fps": 60, "vcodec": "vp9.2", "acodec": "none", "dynamic_range": "HDR10", "tbr": 5500, "filesize": 180000000},
			{"format_id": "140", "protocol": "https", "url": "https://rr1.googlevideo.com/140", "ext": "m4a", "vcodec": "none", "acodec": "mp4a.40.2", "abr": 129.4, "filesize": 10271496, "downloader_options": {"http_chunk_size": 10485760}},
			{"format_id": "140-drc", "format_note": "medium, DRC", "protocol": "https", "url": "https://rr1.googlevideo.com/140drc", "ext": "m4a", "vcodec": "none", "acodec": "mp4a.40.2", "abr": 129.4, "filesize": 10271496},
			{"format_id": "251", "protocol": "https", "url": "https://rr1.googlevideo.com/251", "ext": "webm", "vcodec": "none", "acodec": "opus", "abr": 128.6, "filesize": 10202210}
		]
	}`

	target, _ := ParseTarget("https://www.youtube.com/watch?v=aqz-KE-bpKQ")
	media, err := ParseYTDLPJSON([]byte(fixture), target)
	if err != nil {
		t.Fatalf("ParseYTDLPJSON failed: %v", err)
	}
	if media.SiteID != "youtube" || media.ID != "aqz-KE-bpKQ" || media.Author != "Blender" {
		t.Fatalf("基础元数据不符: %+v", media)
	}
	// 应保留 299(1080p60 SDR AVC)、303(1080p60 SDR VP9)、335(1080p60 HDR10 VP9)；过滤 sb0、628(m3u8)、18(muxed)、248-sr
	if len(media.VideoStreams) != 3 {
		t.Fatalf("期望 3 条独立视频轨，实际 %d: %+v", len(media.VideoStreams), media.VideoStreams)
	}
	if media.VideoStreams[0].StreamKey != "299" || media.VideoStreams[1].StreamKey != "303" {
		t.Fatalf("视频排序错误: %+v", media.VideoStreams)
	}
	if media.VideoStreams[0].QualityGroupKey != media.VideoStreams[1].QualityGroupKey {
		t.Fatalf("同画质不同编码应共享 QualityGroupKey: %q vs %q", media.VideoStreams[0].QualityGroupKey, media.VideoStreams[1].QualityGroupKey)
	}
	if media.VideoStreams[2].QualityGroupKey == media.VideoStreams[0].QualityGroupKey {
		t.Fatalf("HDR 与 SDR 不应错误合并为同一 QualityGroupKey: %q", media.VideoStreams[2].QualityGroupKey)
	}
	if media.VideoStreams[0].Headers["Cookie"] != "" || media.VideoStreams[0].Headers["User-Agent"] != "CustomUA/1.0" {
		t.Fatalf("请求头过滤失败: %+v", media.VideoStreams[0].Headers)
	}
	if media.VideoStreams[0].ChunkSize != 10<<20 {
		t.Fatalf("ChunkSize 不符: %d", media.VideoStreams[0].ChunkSize)
	}
	// 应保留 140 和 251，隐藏重复 140-drc
	if len(media.AudioStreams) != 2 {
		t.Fatalf("期望 2 条音轨，实际 %d: %+v", len(media.AudioStreams), media.AudioStreams)
	}
	if media.AudioStreams[0].StreamKey != "140" || media.AudioStreams[1].StreamKey != "251" {
		t.Fatalf("音轨映射错误: %+v", media.AudioStreams)
	}
}

func TestParseYTDLPJSON_VerticalShortsAndEdgeCases(t *testing.T) {
	t.Run("vertical shorts 1080x1920 labeled 1080P", func(t *testing.T) {
		fixture := `{
			"id": "Short123456",
			"title": "Vertical Short",
			"uploader": "Creator",
			"duration": 15,
			"formats": [
				{"format_id": "299", "protocol": "https", "url": "https://rr1.googlevideo.com/v", "ext": "mp4", "width": 1080, "height": 1920, "fps": 60, "vcodec": "avc1.64002A", "acodec": "none", "filesize": 1000000},
				{"format_id": "140-drc", "format_note": "medium, DRC", "protocol": "https", "url": "https://rr1.googlevideo.com/a", "ext": "m4a", "vcodec": "none", "acodec": "mp4a.40.2", "abr": 128, "filesize": 200000}
			]
		}`
		target, _ := ParseTarget("https://www.youtube.com/shorts/Short123456")
		media, err := ParseYTDLPJSON([]byte(fixture), target)
		if err != nil {
			t.Fatal(err)
		}
		if media.VideoStreams[0].QualityLabel != "1080P 60帧" || media.VideoStreams[0].QualityGroupKey != "1080p-60fps-SDR" {
			t.Fatalf("竖屏 Shorts 画质档位错误: %+v", media.VideoStreams[0])
		}
		// 当仅存在 140-drc 时必须保留且保持独立 StreamKey="140-drc"
		if len(media.AudioStreams) != 1 || media.AudioStreams[0].StreamKey != "140-drc" {
			t.Fatalf("仅 DRC 音轨未正确保留: %+v", media.AudioStreams)
		}
	})

	t.Run("rejects live stream", func(t *testing.T) {
		_, err := ParseYTDLPJSON([]byte(`{"id":"Live1234567","is_live":true}`), Target{VideoID: "Live1234567"})
		if !errors.Is(err, extractor.ErrUnsupportedMedia) {
			t.Fatalf("期望直播返回 ErrUnsupportedMedia, 实际: %v", err)
		}
	})

	t.Run("rejects DRM-only video", func(t *testing.T) {
		_, err := ParseYTDLPJSON([]byte(`{"id":"Drm12345678","formats":[{"format_id":"137","protocol":"https","url":"https://rr1.googlevideo.com/v","has_drm":true,"vcodec":"avc1","acodec":"none"}]}`), Target{VideoID: "Drm12345678"})
		if !errors.Is(err, extractor.ErrUnsupportedMedia) || !strings.Contains(err.Error(), "DRM") {
			t.Fatalf("期望 DRM 返回明确错误, 实际: %v", err)
		}
	})
}

func TestClassifySubprocessError(t *testing.T) {
	cases := []struct {
		stderr  string
		wantErr error
		substr  string
	}{
		{"ERROR: [youtube] id: Sign in to confirm you’re not a bot.", extractor.ErrAPIDenied, "人机校验"},
		{"ERROR: [youtube] id: Private video. Sign in if you've been granted access", extractor.ErrAPIDenied, "私有"},
		{"ERROR: [youtube] id: Video unavailable", extractor.ErrAPIDenied, "不可用"},
		{"ERROR: [youtube] id: Sign in to confirm your age", extractor.ErrAPIDenied, "年龄限制"},
		{"ERROR: [youtube] id: The uploader has not made this video available in your country", extractor.ErrAPIDenied, "地区"},
		{"ERROR: [youtube] id: This video is DRM protected", extractor.ErrUnsupportedMedia, "DRM"},
		{"WARNING: No supported JavaScript runtime could be found", ErrJSRuntimeNotFound, "JavaScript"},
	}
	for _, tc := range cases {
		err := classifySubprocessError(context.Background(), errors.New("exit status 1"), tc.stderr)
		if !errors.Is(err, tc.wantErr) || !strings.Contains(err.Error(), tc.substr) {
			t.Errorf("stderr=%q -> got %v, want %v containing %q", tc.stderr, err, tc.wantErr, tc.substr)
		}
	}
}
