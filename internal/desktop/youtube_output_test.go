package desktop

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"media-fetcher/core/auth"
	"media-fetcher/core/downloader"
	"media-fetcher/core/extractor"
	"media-fetcher/pkg/vault"
)

type youtubeDesktopFixture struct {
	base    string
	vH264   int64
	vVP9    int64
	aAAC    int64
	aOpus   int64
	extract *atomic.Int32
}

func (youtubeDesktopFixture) SiteID() string { return "youtube" }
func (youtubeDesktopFixture) Match(raw string) bool {
	return strings.Contains(raw, "youtube.com/watch")
}
func (f youtubeDesktopFixture) Extract(context.Context, string, extractor.ExtractOptions) (*extractor.MediaInfo, error) {
	f.extract.Add(1)
	return &extractor.MediaInfo{
		SiteID:      "youtube",
		ID:          "aqz-KE-bpKQ",
		Title:       "YouTube Fixture",
		Author:      "Blender",
		SourceURL:   "https://www.youtube.com/watch?v=aqz-KE-bpKQ",
		Type:        extractor.MediaTypeVideo,
		IsAnonymous: true,
		VideoStreams: []extractor.VideoStream{
			{ID: 299, QualityGroupKey: "1080p-60fps-SDR", StreamKey: "299", Container: "mp4", QualityLabel: "1080P 60帧", Width: 1920, Height: 1080, Codec: "avc1.64002A", Bandwidth: 6000000, URL: f.base + "/v_h264.mp4", Size: f.vH264, ChunkSize: 4096, Headers: map[string]string{"User-Agent": "YTTest/1.0"}},
			{ID: 303, QualityGroupKey: "1080p-60fps-SDR", StreamKey: "303", Container: "webm", QualityLabel: "1080P 60帧", Width: 1920, Height: 1080, Codec: "vp9", Bandwidth: 4000000, URL: f.base + "/v_vp9.webm", Size: f.vVP9, ChunkSize: 4096, Headers: map[string]string{"User-Agent": "YTTest/1.0"}},
		},
		AudioStreams: []extractor.AudioStream{
			{ID: 140, StreamKey: "140", Container: "m4a", Kind: "standard", Quality: "128Kbps AAC", Codec: "mp4a.40.2", Bandwidth: 128000, URL: f.base + "/a_aac.m4a", Size: f.aAAC, ChunkSize: 4096, Headers: map[string]string{"User-Agent": "YTTest/1.0"}},
			{ID: 140, StreamKey: "140-drc", Container: "m4a", Kind: "drc", Quality: "128Kbps AAC DRC", Codec: "mp4a.40.2", Bandwidth: 128000, URL: f.base + "/a_aac.m4a", Size: f.aAAC, ChunkSize: 4096, Headers: map[string]string{"User-Agent": "YTTest/1.0"}},
			{ID: 251, StreamKey: "251", Container: "webm", Kind: "standard", Quality: "130Kbps Opus", Codec: "opus", Bandwidth: 130000, URL: f.base + "/a_opus.webm", Size: f.aOpus, ChunkSize: 4096, Headers: map[string]string{"User-Agent": "YTTest/1.0"}},
		},
	}, nil
}

func TestDesktopYouTubeStreamKeyAndLosslessContainers(t *testing.T) {
	ff, err := downloader.FindFFmpeg()
	if err != nil {
		t.Skip("FFmpeg 不可用")
	}
	fixtures := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command(ff, append([]string{"-v", "error", "-y"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg %v: %s", err, out)
		}
	}
	vH264Path := filepath.Join(fixtures, "v_h264.mp4")
	vVP9Path := filepath.Join(fixtures, "v_vp9.webm")
	aAACPath := filepath.Join(fixtures, "a_aac.m4a")
	aOpusPath := filepath.Join(fixtures, "a_opus.webm")
	encOut, encErr := exec.Command(ff, "-hide_banner", "-encoders").CombinedOutput()
	if encErr != nil {
		t.Fatalf("探测 FFmpeg 编码器失败: %v (%s)", encErr, encOut)
	}
	h264Enc := "libx264"
	if !strings.Contains(string(encOut), "libx264") {
		if strings.Contains(string(encOut), "libopenh264") {
			h264Enc = "libopenh264"
		} else {
			t.Fatalf("当前 FFmpeg 缺少测试夹具所需的 H.264 编码器（libx264 或 libopenh264）")
		}
	}
	run("-f", "lavfi", "-i", "color=s=64x64:r=10", "-t", "1", "-an", "-c:v", h264Enc, vH264Path)
	run("-f", "lavfi", "-i", "color=s=64x64:r=10", "-t", "1", "-an", "-c:v", "libvpx-vp9", vVP9Path)
	run("-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "1", "-vn", "-c:a", "aac", aAACPath)
	run("-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "1", "-vn", "-c:a", "libopus", aOpusPath)

	var sawBiliReferer atomic.Bool
	var sawRange atomic.Bool
	fs := http.FileServer(http.Dir(fixtures))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Referer"), "bilibili") {
			sawBiliReferer.Store(true)
		}
		if r.Header.Get("Range") != "" {
			sawRange.Store(true)
		}
		fs.ServeHTTP(w, r)
	}))
	defer server.Close()

	sizeOf := func(p string) int64 {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		return st.Size()
	}

	var extractCalls atomic.Int32
	reg := extractor.NewRegistry()
	reg.Register(youtubeDesktopFixture{
		base:    server.URL,
		vH264:   sizeOf(vH264Path),
		vVP9:    sizeOf(vVP9Path),
		aAAC:    sizeOf(aAACPath),
		aOpus:   sizeOf(aOpusPath),
		extract: &extractCalls,
	})
	cred, err := auth.NewCredentialService(t.TempDir(), vault.NewMockVault("yt-desktop"))
	if err != nil {
		t.Fatal(err)
	}
	s := &DesktopService{registry: reg, credService: cred, historyDir: t.TempDir()}
	outDir := t.TempDir()

	cases := []struct {
		name, mode, vKey, vCodec, aKey, aFmt, wantExt string
		vID, aID                                      int
	}{
		{"mp4_complete", "complete", "299", "avc1.64002A", "140", "", "mp4", 299, 140},
		{"webm_complete", "complete", "303", "vp9", "251", "", "webm", 303, 251},
		{"mkv_cross", "complete", "299", "avc1.64002A", "251", "", "mkv", 299, 251},
		{"drc_audio_distinct", "audio", "", "", "140-drc", "original", "m4a", -1, 140},
		{"opus_original", "audio", "", "", "251", "original", "opus", -1, 251},
		{"vp9_video_only", "video", "303", "vp9", "", "", "webm", 303, -1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := DownloadParams{
				TaskID:      "0123456789abcdef0123456789abcdef",
				URL:         "https://www.youtube.com/watch?v=aqz-KE-bpKQ&si=tracking",
				Mode:        tc.mode,
				AudioFormat: tc.aFmt,
				OutputDir:   outDir,
				VideoID:     tc.vID,
				VideoCodec:  tc.vCodec,
				VideoKey:    tc.vKey,
				AudioID:     tc.aID,
				AudioKey:    tc.aKey,
			}
			if err := validateDownload(p); err != nil {
				t.Fatalf("validateDownload failed: %v", err)
			}
			if err := s.doDownload(context.Background(), p); err != nil {
				t.Fatalf("doDownload failed: %v", err)
			}
			hist, err := s.DownloadHistory()
			if err != nil || len(hist) == 0 {
				t.Fatalf("history error: %v", err)
			}
			rec := hist[0]
			if rec.Format != tc.wantExt {
				t.Fatalf("format got %q, want %q", rec.Format, tc.wantExt)
			}
			if rec.SourceURL != "https://www.youtube.com/watch?v=aqz-KE-bpKQ" {
				t.Fatalf("sourceUrl broken: %q", rec.SourceURL)
			}
		})
	}
	if sawBiliReferer.Load() {
		t.Fatal("YouTube 下载请求错误携带了 Bilibili Referer")
	}
	if !sawRange.Load() {
		t.Fatal("YouTube 分块轨道未触发 HTTP Range 请求")
	}

	// 验证无效 StreamKey 不会静默回退到其他轨道
	bad := DownloadParams{
		TaskID:    "0123456789abcdef0123456789abcdef",
		URL:       "https://www.youtube.com/watch?v=aqz-KE-bpKQ",
		Mode:      "video",
		OutputDir: outDir,
		VideoID:   299,
		VideoKey:  "non-existent-itag",
		AudioID:   -1,
	}
	if err := s.doDownload(context.Background(), bad); err == nil {
		t.Fatal("不存在的 VideoKey 不得静默回退")
	}
}
