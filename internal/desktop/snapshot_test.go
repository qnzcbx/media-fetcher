package desktop

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"media-fetcher/core/auth"
	"media-fetcher/core/downloader"
	"media-fetcher/core/extractor"
	"media-fetcher/pkg/cookiejar"
	"media-fetcher/pkg/vault"
)

type countingExtractor struct {
	siteID    string
	calls     atomic.Int32
	onExtract func(call int32, rawURL string) (*extractor.MediaInfo, error)
}

func (c *countingExtractor) SiteID() string        { return c.siteID }
func (c *countingExtractor) Match(raw string) bool { return strings.Contains(raw, c.siteID) }
func (c *countingExtractor) Extract(_ context.Context, rawURL string, _ extractor.ExtractOptions) (*extractor.MediaInfo, error) {
	n := c.calls.Add(1)
	return c.onExtract(n, rawURL)
}

func makeSampleMediaFiles(t *testing.T, dir string) (videoBytes, audioBytes []byte) {
	t.Helper()
	ff, err := downloader.FindFFmpeg()
	if err != nil {
		t.Skip("FFmpeg 不可用")
	}
	vPath := filepath.Join(dir, "v.webm")
	aPath := filepath.Join(dir, "a.webm")
	if out, err := exec.Command(ff, "-v", "error", "-y", "-f", "lavfi", "-i", "color=s=64x64:r=10", "-t", "1", "-an", "-c:v", "libvpx-vp9", vPath).CombinedOutput(); err != nil {
		t.Fatalf("gen video: %v %s", err, out)
	}
	if out, err := exec.Command(ff, "-v", "error", "-y", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "1", "-vn", "-c:a", "libopus", aPath).CombinedOutput(); err != nil {
		t.Fatalf("gen audio: %v %s", err, out)
	}
	vb, err := os.ReadFile(vPath)
	if err != nil {
		t.Fatal(err)
	}
	ab, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}
	return vb, ab
}

func TestExtractSnapshot_ReuseTTLExpiryInvalidationAndFallback(t *testing.T) {
	tmp := t.TempDir()
	videoBytes, audioBytes := makeSampleMediaFiles(t, tmp)

	var expiredHit atomic.Int32
	var freshHit atomic.Int32
	var always403Hit atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/expired-video":
			expiredHit.Add(1)
			http.Error(w, "gone", http.StatusGone)
		case "/always-403":
			always403Hit.Add(1)
			http.Error(w, "forbidden", http.StatusForbidden)
		case "/video":
			freshHit.Add(1)
			serveBytesRange(w, r, videoBytes)
		case "/audio":
			serveBytesRange(w, r, audioBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	newSvc := func(ext extractor.Extractor, nowFn func() time.Time) *DesktopService {
		cred, err := auth.NewCredentialService(filepath.Join(tmp, fmt.Sprintf("cred-%d", time.Now().UnixNano())), vault.NewDefaultVault())
		if err != nil {
			t.Fatal(err)
		}
		reg := extractor.NewRegistry()
		reg.Register(ext)
		return &DesktopService{
			credService: cred,
			registry:    reg,
			historyDir:  filepath.Join(tmp, fmt.Sprintf("hist-%d", time.Now().UnixNano())),
			snapshotTTL: 90 * time.Second,
			snapshotNow: nowFn,
		}
	}

	const watchURL = "https://www.youtube.com/watch?v=aqz-KE-bpKQ"

	t.Run("reuses fresh snapshot without second extract and preserves exact StreamKey", func(t *testing.T) {
		curTime := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
		futureExpire := curTime.Add(30 * time.Minute).Unix()
		ext := &countingExtractor{
			siteID: "youtube",
			onExtract: func(_ int32, _ string) (*extractor.MediaInfo, error) {
				return &extractor.MediaInfo{
					SiteID:    "youtube",
					ID:        "aqz-KE-bpKQ",
					Title:     "Snapshot Reuse Test",
					SourceURL: watchURL,
					VideoStreams: []extractor.VideoStream{
						{ID: 244, StreamKey: "244", Codec: "vp9", Container: "webm", Size: int64(len(videoBytes)), ChunkSize: 10 << 20, URL: fmt.Sprintf("%s/video?expire=%d", srv.URL, futureExpire)},
					},
					AudioStreams: []extractor.AudioStream{
						{ID: 251, StreamKey: "251", Codec: "opus", Container: "webm", Size: int64(len(audioBytes)), ChunkSize: 10 << 20, URL: fmt.Sprintf("%s/audio?expire=%d", srv.URL, futureExpire)},
					},
				}, nil
			},
		}
		svc := newSvc(ext, func() time.Time { return curTime })

		if _, err := svc.Extract(watchURL, ""); err != nil {
			t.Fatal(err)
		}
		if ext.calls.Load() != 1 {
			t.Fatalf("expected 1 extract call, got %d", ext.calls.Load())
		}

		outDir := filepath.Join(tmp, "out-reuse")
		_ = os.MkdirAll(outDir, 0755)
		err := svc.doDownload(context.Background(), DownloadParams{
			URL: watchURL, Mode: "complete", OutputDir: outDir,
			VideoID: 244, VideoKey: "244", VideoCodec: "vp9",
			AudioID: 251, AudioKey: "251",
		})
		if err != nil {
			t.Fatalf("doDownload failed: %v", err)
		}
		// 命中短 TTL 快照，未执行第二次 Extract
		if ext.calls.Load() != 1 {
			t.Fatalf("expected snapshot reuse (1 extract total), got %d", ext.calls.Load())
		}
	})

	t.Run("expires after TTL and re-extracts", func(t *testing.T) {
		curTime := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
		ext := &countingExtractor{
			siteID: "youtube",
			onExtract: func(_ int32, _ string) (*extractor.MediaInfo, error) {
				return &extractor.MediaInfo{
					SiteID:    "youtube",
					ID:        "aqz-KE-bpKQ",
					Title:     "TTL Test",
					SourceURL: watchURL,
					VideoStreams: []extractor.VideoStream{
						{ID: 244, StreamKey: "244", Codec: "vp9", Container: "webm", Size: int64(len(videoBytes)), ChunkSize: 10 << 20, URL: srv.URL + "/video"},
					},
					AudioStreams: []extractor.AudioStream{
						{ID: 251, StreamKey: "251", Codec: "opus", Container: "webm", Size: int64(len(audioBytes)), ChunkSize: 10 << 20, URL: srv.URL + "/audio"},
					},
				}, nil
			},
		}
		svc := newSvc(ext, func() time.Time { return curTime })
		if _, err := svc.Extract(watchURL, ""); err != nil {
			t.Fatal(err)
		}
		// 前进超过 90s TTL
		curTime = curTime.Add(95 * time.Second)
		outDir := filepath.Join(tmp, "out-ttl")
		_ = os.MkdirAll(outDir, 0755)
		if err := svc.doDownload(context.Background(), DownloadParams{
			URL: watchURL, Mode: "complete", OutputDir: outDir,
			VideoID: 244, VideoKey: "244", VideoCodec: "vp9",
			AudioID: 251, AudioKey: "251",
		}); err != nil {
			t.Fatal(err)
		}
		if ext.calls.Load() != 2 {
			t.Fatalf("expected re-extract after TTL expiry (2 calls), got %d", ext.calls.Load())
		}
	})

	t.Run("rejects soon-expiring signed URL before dialing", func(t *testing.T) {
		curTime := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
		soonExpire := curTime.Add(30 * time.Second).Unix()
		ext := &countingExtractor{
			siteID: "youtube",
			onExtract: func(n int32, _ string) (*extractor.MediaInfo, error) {
				u := srv.URL + "/video"
				if n == 1 {
					u = fmt.Sprintf("%s/expired-video?expire=%d", srv.URL, soonExpire)
				}
				return &extractor.MediaInfo{
					SiteID:    "youtube",
					ID:        "aqz-KE-bpKQ",
					Title:     "Expire Query Test",
					SourceURL: watchURL,
					VideoStreams: []extractor.VideoStream{
						{ID: 244, StreamKey: "244", Codec: "vp9", Container: "webm", Size: int64(len(videoBytes)), ChunkSize: 10 << 20, URL: u},
					},
					AudioStreams: []extractor.AudioStream{
						{ID: 251, StreamKey: "251", Codec: "opus", Container: "webm", Size: int64(len(audioBytes)), ChunkSize: 10 << 20, URL: srv.URL + "/audio"},
					},
				}, nil
			},
		}
		expiredHit.Store(0)
		svc := newSvc(ext, func() time.Time { return curTime })
		if _, err := svc.Extract(watchURL, ""); err != nil {
			t.Fatal(err)
		}
		outDir := filepath.Join(tmp, "out-expire")
		_ = os.MkdirAll(outDir, 0755)
		if err := svc.doDownload(context.Background(), DownloadParams{
			URL: watchURL, Mode: "complete", OutputDir: outDir,
			VideoID: 244, VideoKey: "244", VideoCodec: "vp9",
			AudioID: 251, AudioKey: "251",
		}); err != nil {
			t.Fatal(err)
		}
		if expiredHit.Load() != 0 || ext.calls.Load() != 2 {
			t.Fatalf("expected pre-dial expiry skip & re-extract (expiredHit=%d calls=%d)", expiredHit.Load(), ext.calls.Load())
		}
	})

	t.Run("falls back once on snapshot 410 and recovers with fresh extract", func(t *testing.T) {
		curTime := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
		ext := &countingExtractor{
			siteID: "youtube",
			onExtract: func(n int32, _ string) (*extractor.MediaInfo, error) {
				u := srv.URL + "/video"
				if n == 1 {
					u = srv.URL + "/expired-video"
				}
				return &extractor.MediaInfo{
					SiteID:    "youtube",
					ID:        "aqz-KE-bpKQ",
					Title:     "Fallback Test",
					SourceURL: watchURL,
					VideoStreams: []extractor.VideoStream{
						{ID: 244, StreamKey: "244", Codec: "vp9", Container: "webm", Size: int64(len(videoBytes)), ChunkSize: 10 << 20, URL: u},
					},
					AudioStreams: []extractor.AudioStream{
						{ID: 251, StreamKey: "251", Codec: "opus", Container: "webm", Size: int64(len(audioBytes)), ChunkSize: 10 << 20, URL: srv.URL + "/audio"},
					},
				}, nil
			},
		}
		expiredHit.Store(0)
		svc := newSvc(ext, func() time.Time { return curTime })
		if _, err := svc.Extract(watchURL, ""); err != nil {
			t.Fatal(err)
		}
		outDir := filepath.Join(tmp, "out-fallback")
		_ = os.MkdirAll(outDir, 0755)
		if err := svc.doDownload(context.Background(), DownloadParams{
			URL: watchURL, Mode: "complete", OutputDir: outDir,
			VideoID: 244, VideoKey: "244", VideoCodec: "vp9",
			AudioID: 251, AudioKey: "251",
		}); err != nil {
			t.Fatalf("expected fallback recovery, got err: %v", err)
		}
		if expiredHit.Load() != 1 || ext.calls.Load() != 2 {
			t.Fatalf("expected 1 snapshot attempt + 1 fresh extract, got expiredHit=%d calls=%d", expiredHit.Load(), ext.calls.Load())
		}
	})

	t.Run("persistent 403 after fresh extract fails cleanly without infinite retry", func(t *testing.T) {
		curTime := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
		ext := &countingExtractor{
			siteID: "youtube",
			onExtract: func(_ int32, _ string) (*extractor.MediaInfo, error) {
				return &extractor.MediaInfo{
					SiteID:    "youtube",
					ID:        "aqz-KE-bpKQ",
					Title:     "Persistent 403",
					SourceURL: watchURL,
					VideoStreams: []extractor.VideoStream{
						{ID: 244, StreamKey: "244", Codec: "vp9", Container: "webm", Size: int64(len(videoBytes)), ChunkSize: 10 << 20, URL: srv.URL + "/always-403"},
					},
					AudioStreams: []extractor.AudioStream{
						{ID: 251, StreamKey: "251", Codec: "opus", Container: "webm", Size: int64(len(audioBytes)), ChunkSize: 10 << 20, URL: srv.URL + "/audio"},
					},
				}, nil
			},
		}
		always403Hit.Store(0)
		svc := newSvc(ext, func() time.Time { return curTime })
		if _, err := svc.Extract(watchURL, ""); err != nil {
			t.Fatal(err)
		}
		outDir := filepath.Join(tmp, "out-403")
		_ = os.MkdirAll(outDir, 0755)
		err := svc.doDownload(context.Background(), DownloadParams{
			URL: watchURL, Mode: "complete", OutputDir: outDir,
			VideoID: 244, VideoKey: "244", VideoCodec: "vp9",
			AudioID: 251, AudioKey: "251",
		})
		if err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("expected real 403 error, got %v", err)
		}
		// 仅允许单次回退重新解析（总调用次数 = 2，不循环）
		if ext.calls.Load() != 2 {
			t.Fatalf("expected exactly 2 extract calls (1 initial + 1 fallback), got %d", ext.calls.Load())
		}
	})

	t.Run("logout, resource switch, and close invalidate snapshot; bilibili never uses snapshot", func(t *testing.T) {
		curTime := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
		const biliURL = "https://www.bilibili.com/video/BV1xx411c7mD"
		const watchURL2 = "https://www.youtube.com/watch?v=jNQXAC9IVRw"
		ytExt := &countingExtractor{
			siteID: "youtube",
			onExtract: func(_ int32, raw string) (*extractor.MediaInfo, error) {
				return &extractor.MediaInfo{
					SiteID:    "youtube",
					ID:        "aqz-KE-bpKQ",
					Title:     "YT Session",
					SourceURL: raw,
					VideoStreams: []extractor.VideoStream{
						{ID: 244, StreamKey: "244", Codec: "vp9", URL: srv.URL + "/video"},
					},
				}, nil
			},
		}
		biliExt := &countingExtractor{
			siteID: "bilibili",
			onExtract: func(_ int32, _ string) (*extractor.MediaInfo, error) {
				return &extractor.MediaInfo{
					SiteID:    "bilibili",
					ID:        "BV1xx411c7mD",
					Title:     "Bili Auth",
					SourceURL: biliURL,
					VideoStreams: []extractor.VideoStream{
						{ID: 80, Codec: "avc1", URL: srv.URL + "/video"},
					},
				}, nil
			},
		}
		svc := newSvc(ytExt, func() time.Time { return curTime })
		svc.registry.Register(biliExt)
		jar := cookiejar.New("youtube", "u1")
		if err := svc.credService.RegisterOrUpdateAccount(&auth.Account{SiteID: "youtube", AccountID: "u1", Name: "Tester"}, jar); err != nil {
			t.Fatal(err)
		}
		// Bilibili 永不复用快照（保持原重解析策略不变）
		if _, err := svc.Extract(biliURL, ""); err != nil {
			t.Fatal(err)
		}
		if got := svc.lookupExtractSnapshot("bilibili", biliURL, ""); got != nil {
			t.Fatal("bilibili must not use extract snapshot")
		}
		// YouTube 账号退出立即失效快照
		if _, err := svc.Extract(watchURL, "u1"); err != nil {
			t.Fatal(err)
		}
		if got := svc.lookupExtractSnapshot("youtube", watchURL, "u1"); got == nil {
			t.Fatal("expected valid youtube snapshot before logout")
		}
		if err := svc.LogoutAccount("youtube", "u1"); err != nil {
			t.Fatal(err)
		}
		if got := svc.lookupExtractSnapshot("youtube", watchURL, "u1"); got != nil {
			t.Fatal("expected snapshot invalidated after LogoutAccount")
		}
		// 切换资源自动淘汰旧资源快照
		if _, err := svc.Extract(watchURL, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Extract(watchURL2, ""); err != nil {
			t.Fatal(err)
		}
		if got := svc.lookupExtractSnapshot("youtube", watchURL, ""); got != nil {
			t.Fatal("expected previous resource snapshot evicted on resource switch")
		}
		// ClearExtractSnapshot 立即清除
		svc.ClearExtractSnapshot()
		if got := svc.lookupExtractSnapshot("youtube", watchURL2, ""); got != nil {
			t.Fatal("expected snapshot cleared after ClearExtractSnapshot")
		}
	})
}

func serveBytesRange(w http.ResponseWriter, r *http.Request, data []byte) {
	total := len(data)
	rng := r.Header.Get("Range")
	if strings.HasPrefix(rng, "bytes=") {
		parts := strings.Split(strings.TrimPrefix(rng, "bytes="), "-")
		start, _ := strconv.Atoi(parts[0])
		end := total - 1
		if len(parts) > 1 && parts[1] != "" {
			if e, err := strconv.Atoi(parts[1]); err == nil && e < total {
				end = e
			}
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
		w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data[start : end+1])
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(total))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
