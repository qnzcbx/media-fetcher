package desktop

import (
	"bytes"
	"context"
	"encoding/base64"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"media-fetcher/core/auth"
	"media-fetcher/core/downloader"
	"media-fetcher/core/extractor"
	"media-fetcher/pkg/vault"
)

type trackFixture struct{ base string }

func (trackFixture) SiteID() string    { return "bilibili" }
func (trackFixture) Match(string) bool { return true }
func (f trackFixture) Extract(context.Context, string, extractor.ExtractOptions) (*extractor.MediaInfo, error) {
	return &extractor.MediaInfo{Title: "fixture", SiteID: "bilibili",
		VideoStreams: []extractor.VideoStream{{ID: 80, Codec: "avc1", URL: f.base + "/video.m4s"}},
		AudioStreams: []extractor.AudioStream{{ID: 30280, URL: f.base + "/audio.m4s"}}}, nil
}

func TestDesktopMediaOutputs(t *testing.T) {
	ffmpeg, err := downloader.FindFFmpeg()
	if err != nil {
		t.Skip("FFmpeg 不可用")
	}
	fixtures := t.TempDir()
	for _, item := range []struct {
		name string
		args []string
	}{
		{"video.m4s", []string{"-f", "lavfi", "-i", "color=s=32x32:r=10", "-t", "1", "-an", "-c:v", "mpeg4"}},
		{"audio.m4s", []string{"-f", "lavfi", "-i", "sine=frequency=440", "-t", "1", "-vn", "-c:a", "aac"}},
	} {
		args := append([]string{"-v", "error"}, item.args...)
		output, err := exec.Command(ffmpeg, append(args, "-f", "mp4", filepath.Join(fixtures, item.name))...).CombinedOutput()
		if err != nil {
			t.Fatalf("生成测试轨道: %v %s", err, output)
		}
	}
	if _, err = downloader.ProbeMedia(t.Context(), filepath.Join(fixtures, "video.m4s")); err != nil {
		t.Skip("FFprobe 不可用")
	}
	counts := map[string]int{}
	var mu sync.Mutex
	files := http.FileServer(http.Dir(fixtures))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		counts[r.URL.Path]++
		mu.Unlock()
		files.ServeHTTP(w, r)
	}))
	defer server.Close()
	registry := extractor.NewRegistry()
	registry.Register(trackFixture{server.URL})
	cred, err := auth.NewCredentialService(t.TempDir(), vault.NewMockVault("outputs"))
	if err != nil {
		t.Fatal(err)
	}
	s := &DesktopService{registry: registry, credService: cred, historyDir: t.TempDir()}
	out := t.TempDir()
	for _, tc := range []struct {
		mode, format, suffix      string
		video, audio              int
		streams, codec, container string
	}{
		{"complete", "", "mp4", 80, 30280, "audio,video", "", "mp4"},
		{"video", "", "mp4", 80, -1, "video", "", "mp4"},
		{"audio", "m4a", "m4a", -1, 30280, "audio", "aac", "mp4"},
		{"audio", "mp3", "mp3", -1, 30280, "audio", "mp3", "mp3"},
		{"audio", "m4a", "m4a", -1, 30280, "audio", "aac", "mp4"},
	} {
		t.Run(tc.mode+tc.format, func(t *testing.T) {
			mu.Lock()
			clear(counts)
			mu.Unlock()
			p := DownloadParams{TaskID: "0123456789abcdef0123456789abcdef", URL: "https://www.bilibili.com/video/BV1xx411c7mD", Mode: tc.mode, AudioFormat: tc.format, OutputDir: out, VideoID: tc.video, AudioID: tc.audio, VideoCodec: "avc1"}
			if err := validateDownload(p); err != nil {
				t.Fatal(err)
			}
			if err := s.doDownload(t.Context(), p); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			v, a := counts["/video.m4s"], counts["/audio.m4s"]
			mu.Unlock()
			if (v == 1) != (tc.video >= 0) || (a == 1) != (tc.audio >= 0) || v > 1 || a > 1 {
				t.Fatalf("轨道请求不匹配: %d %d", v, a)
			}
			history, err := s.DownloadHistory()
			if err != nil || len(history) == 0 {
				t.Fatalf("历史: %v", err)
			}
			r := history[0]
			if r.Format != tc.suffix || r.DurationSeconds <= 0 {
				t.Fatalf("元数据: %+v", r)
			}
			info, err := os.Stat(r.FinalPath)
			if err != nil || r.FileSize != info.Size() {
				t.Fatalf("大小: %+v, %v", r, err)
			}
			probe, err := downloader.ProbeMedia(t.Context(), r.FinalPath)
			if err != nil || len(probe.Streams) != len(strings.Split(tc.streams, ",")) || !strings.Contains(probe.Format.Name, tc.container) {
				t.Fatalf("输出: %+v, %v", probe, err)
			}
			for _, stream := range probe.Streams {
				if !strings.Contains(tc.streams, stream.Type) || tc.codec != "" && tc.codec != stream.Codec {
					t.Fatalf("错误轨道: %+v", stream)
				}
			}
		})
	}
	records, err := s.DownloadHistory()
	if err != nil || len(records) != 5 {
		t.Fatalf("记录数: %v, %v", len(records), err)
	}
	paths := map[string]bool{}
	for _, r := range records {
		if paths[r.FinalPath] {
			t.Fatalf("输出冲突: %s", r.FinalPath)
		}
		paths[r.FinalPath] = true
	}
}

func TestPublishUniquePreservesExistingFile(t *testing.T) {
	d := t.TempDir()
	src, dst := filepath.Join(d, "source"), filepath.Join(d, "result.mp4")
	for path, contents := range map[string]string{src: "new", dst: "old"} {
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := publishUnique(t.Context(), src, dst)
	if err != nil || filepath.Base(got) != "result (1).mp4" {
		t.Fatalf("%q %v", got, err)
	}
	f, err := os.Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil || string(data) != "old" {
		t.Fatalf("旧文件: %q %v", data, err)
	}
}

func TestInvalidDesktopMediaModes(t *testing.T) {
	for _, p := range []DownloadParams{
		{Mode: "audio", AudioID: 1, VideoID: 1, AudioFormat: "mp3"},
		{Mode: "audio", AudioID: 1, VideoID: -1, AudioFormat: "wav"},
		{Mode: "video", AudioID: 1, VideoID: 1},
		{Mode: "complete", AudioID: -1, VideoID: -1},
	} {
		if err := (&DesktopService{}).StartDownload(p); err == nil {
			t.Fatal("无效组合被接受")
		}
	}
}

func TestLegacyDownloadMetadata(t *testing.T) {
	dir := t.TempDir()
	exists := filepath.Join(dir, "old.m4a")
	if err := os.WriteFile(exists, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeHistory(dir, []DownloadRecord{{FinalPath: exists}, {FinalPath: filepath.Join(dir, "gone.mp4")}}); err != nil {
		t.Fatal(err)
	}
	records, err := readHistory(dir)
	if err != nil || len(records) != 2 {
		t.Fatalf("%v %v", records, err)
	}
	if records[0].FileSize != 4 || records[0].Format != "m4a" || records[0].DurationSeconds != 0 || records[1].FileSize != 0 {
		t.Fatalf("%+v", records)
	}
}

func TestWebPThumbnail(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString("UklGRiQAAABXRUJQVlA4IBgAAAAwAQCdASoIAAgAAgA0JaQAA3AA/vv9UAA=")
	if err != nil {
		t.Fatal(err)
	}
	thumbnail, err := makeThumbnail(data)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(thumbnail))
	if err != nil || decoded.Bounds().Dx() != 8 || decoded.Bounds().Dy() != 8 {
		t.Fatalf("缩略图解码失败: %v", err)
	}
}
