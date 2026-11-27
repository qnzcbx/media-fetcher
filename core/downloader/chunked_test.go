package downloader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func parseTestRangeHeader(h string) (int64, int64, bool) {
	if !strings.HasPrefix(h, "bytes=") {
		return 0, 0, false
	}
	parts := strings.Split(strings.TrimPrefix(h, "bytes="), "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	s, err1 := strconv.ParseInt(parts[0], 10, 64)
	e, err2 := strconv.ParseInt(parts[1], 10, 64)
	return s, e, err1 == nil && err2 == nil
}

func TestDownloader_ChunkedRangeSuccessAndBoundaries(t *testing.T) {
	data := make([]byte, 25000)
	for i := range data {
		data[i] = byte(i % 251)
	}
	var gotRanges []string
	var sawUnsafeHeader atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			sawUnsafeHeader.Store(true)
		}
		if r.Header.Get("Accept-Language") != "en-US" {
			http.Error(w, "missing safe header", http.StatusBadRequest)
			return
		}
		rng := r.Header.Get("Range")
		s, e, ok := parseTestRangeHeader(rng)
		if !ok {
			http.Error(w, "bad range", http.StatusBadRequest)
			return
		}
		gotRanges = append(gotRanges, rng)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", s, e, len(data)))
		w.Header().Set("Content-Length", strconv.FormatInt(e-s+1, 10))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data[s : e+1])
	}))
	defer server.Close()

	d := New(nil)
	outDir := t.TempDir()
	var prev int64
	var progressBad bool
	saved, err := d.Download(context.Background(), nil, server.URL, DownloadOptions{
		OutputDir: outDir,
		Filename:  "chunked.bin",
		ChunkSize: 8000,
		Headers: map[string]string{
			"Accept-Language": "en-US",
			"Cookie":          "evil=1",
			"Authorization":   "Bearer secret",
		},
		RetryDelay: time.Millisecond,
		OnProgress: func(downloaded, total int64) {
			if downloaded < prev || total != int64(len(data)) {
				progressBad = true
			}
			prev = downloaded
		},
	})
	if err != nil {
		t.Fatalf("分块下载失败: %v", err)
	}
	if sawUnsafeHeader.Load() {
		t.Fatal("白名单外的敏感请求头被透传")
	}
	if progressBad || prev != int64(len(data)) {
		t.Fatalf("进度异常: prev=%d bad=%v", prev, progressBad)
	}
	// 首请求为 bytes=0-0 探测，随后 4 块覆盖 [0..7999], [8000..15999], [16000..23999], [24000..24999]
	wantRanges := []string{"bytes=0-0", "bytes=0-7999", "bytes=8000-15999", "bytes=16000-23999", "bytes=24000-24999"}
	if strings.Join(gotRanges, ",") != strings.Join(wantRanges, ",") {
		t.Fatalf("分块边界不匹配: got=%v want=%v", gotRanges, wantRanges)
	}
	got, err := os.ReadFile(saved)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("拼接内容与源数据不符: err=%v len=%d", err, len(got))
	}
	if parts, _ := filepath.Glob(filepath.Join(outDir, "*.part*")); len(parts) > 0 {
		t.Fatalf("残留临时文件: %v", parts)
	}
}

func TestDownloader_ChunkedRetryNoDuplicateBytesOrProgress(t *testing.T) {
	data := bytes.Repeat([]byte("0123456789"), 300) // 3000 字节
	var chunk2Attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, e, ok := parseTestRangeHeader(r.Header.Get("Range"))
		if !ok {
			http.Error(w, "bad range", http.StatusBadRequest)
			return
		}
		if s == 1000 && chunk2Attempts.Add(1) == 1 {
			// 第 2 块首次故意只写一半后断流
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", s, e, len(data)))
			w.Header().Set("Content-Length", strconv.FormatInt(e-s+1, 10))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(data[s : s+400])
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", s, e, len(data)))
		w.Header().Set("Content-Length", strconv.FormatInt(e-s+1, 10))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data[s : e+1])
	}))
	defer server.Close()

	d := New(nil)
	outDir := t.TempDir()
	var prev int64
	saved, err := d.Download(context.Background(), nil, server.URL, DownloadOptions{
		OutputDir:    outDir,
		Filename:     "retry.bin",
		ChunkSize:    1000,
		ExpectedSize: int64(len(data)),
		MaxRetries:   2,
		RetryDelay:   time.Millisecond,
		OnProgress: func(downloaded, total int64) {
			if downloaded < prev || downloaded > total {
				t.Errorf("重试导致进度倒退或溢出: prev=%d cur=%d total=%d", prev, downloaded, total)
			}
			prev = downloaded
		},
	})
	if err != nil {
		t.Fatalf("分块重试后仍失败: %v", err)
	}
	if chunk2Attempts.Load() != 2 {
		t.Fatalf("期望第 2 块重试 1 次，实际请求次数: %d", chunk2Attempts.Load())
	}
	got, _ := os.ReadFile(saved)
	if !bytes.Equal(got, data) {
		t.Fatalf("重试后文件内容损坏: got len=%d want=%d", len(got), len(data))
	}
}

func TestDownloader_ChunkedErrorRejections(t *testing.T) {
	t.Run("server ignores range with 200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("full body"))
		}))
		defer server.Close()
		outDir := t.TempDir()
		_, err := New(nil).Download(context.Background(), nil, server.URL, DownloadOptions{
			OutputDir: outDir, Filename: "ignore.bin", ChunkSize: 4, ExpectedSize: 9, RetryDelay: time.Millisecond,
		})
		if !errors.Is(err, ErrInvalidHTTPStatus) {
			t.Fatalf("期望 ErrInvalidHTTPStatus, 实际: %v", err)
		}
		if parts, _ := filepath.Glob(filepath.Join(outDir, "*.part*")); len(parts) > 0 {
			t.Fatalf("残留临时文件: %v", parts)
		}
	})

	t.Run("bad content-range rejected", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Range", "bytes 0-9/*")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(bytes.Repeat([]byte("a"), 10))
		}))
		defer server.Close()
		_, err := New(nil).Download(context.Background(), nil, server.URL, DownloadOptions{
			OutputDir: t.TempDir(), Filename: "badcr.bin", ChunkSize: 10, ExpectedSize: 10, RetryDelay: time.Millisecond,
		})
		if !errors.Is(err, ErrInvalidContentRange) {
			t.Fatalf("期望 ErrInvalidContentRange, 实际: %v", err)
		}
	})

	t.Run("file size changed mid-download rejected", func(t *testing.T) {
		var reqs atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, e, _ := parseTestRangeHeader(r.Header.Get("Range"))
			total := 20
			if reqs.Add(1) > 1 {
				total = 25 // 第二块总大小突变
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", s, e, total))
			w.Header().Set("Content-Length", strconv.FormatInt(e-s+1, 10))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(bytes.Repeat([]byte("x"), int(e-s+1)))
		}))
		defer server.Close()
		outDir := t.TempDir()
		_, err := New(nil).Download(context.Background(), nil, server.URL, DownloadOptions{
			OutputDir: outDir, Filename: "changed.bin", ChunkSize: 10, ExpectedSize: 20, RetryDelay: time.Millisecond,
		})
		if !errors.Is(err, ErrRangeMismatch) {
			t.Fatalf("期望 ErrRangeMismatch, 实际: %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(outDir, "changed.bin")); !os.IsNotExist(statErr) {
			t.Fatal("大小变化后不应发布正式文件")
		}
	})

	t.Run("403 fails immediately without blind retries", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()
		_, err := New(nil).Download(context.Background(), nil, server.URL, DownloadOptions{
			OutputDir: t.TempDir(), Filename: "forbidden.bin", ChunkSize: 1024, ExpectedSize: 4096, MaxRetries: 3, RetryDelay: time.Millisecond,
		})
		if !errors.Is(err, ErrInvalidHTTPStatus) {
			t.Fatalf("期望 ErrInvalidHTTPStatus, 实际: %v", err)
		}
		if calls.Load() != 1 {
			t.Fatalf("403 不应盲目循环重试，实际请求次数: %d", calls.Load())
		}
	})

	t.Run("403 large first chunk falls back to 2MiB once", func(t *testing.T) {
		total := int64(3 << 20)
		var largeCalls, smallCalls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, e, _ := parseTestRangeHeader(r.Header.Get("Range"))
			if e-s+1 > fallbackSmallChunk {
				largeCalls.Add(1)
				w.WriteHeader(http.StatusForbidden)
				return
			}
			smallCalls.Add(1)
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", s, e, total))
			w.Header().Set("Content-Length", strconv.FormatInt(e-s+1, 10))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(bytes.Repeat([]byte("z"), int(e-s+1)))
		}))
		defer server.Close()
		saved, err := New(nil).Download(context.Background(), nil, server.URL, DownloadOptions{
			OutputDir: t.TempDir(), Filename: "fallback2m.bin", ChunkSize: 10 << 20, ExpectedSize: total, RetryDelay: time.Millisecond,
		})
		if err != nil {
			t.Fatalf("2MiB 回退下载失败: %v", err)
		}
		if largeCalls.Load() != 1 || smallCalls.Load() != 2 {
			t.Fatalf("期望大块 1 次 + 小块 2 次，实际 large=%d small=%d", largeCalls.Load(), smallCalls.Load())
		}
		info, _ := os.Stat(saved)
		if info.Size() != total {
			t.Fatalf("大小不符: %d != %d", info.Size(), total)
		}
	})

	t.Run("cancellation stops subsequent chunks and cleans part", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, e, _ := parseTestRangeHeader(r.Header.Get("Range"))
			if calls.Add(1) == 1 {
				cancel() // 首块完成后立即取消
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/300", s, e))
			w.Header().Set("Content-Length", strconv.FormatInt(e-s+1, 10))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(bytes.Repeat([]byte("c"), int(e-s+1)))
		}))
		defer server.Close()
		outDir := t.TempDir()
		_, err := New(nil).Download(ctx, nil, server.URL, DownloadOptions{
			OutputDir: outDir, Filename: "cancel_chunk.bin", ChunkSize: 100, ExpectedSize: 300, RetryDelay: time.Millisecond,
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("期望 context.Canceled, 实际: %v", err)
		}
		if calls.Load() != 1 {
			t.Fatalf("取消后不应发起后续分块请求，实际: %d", calls.Load())
		}
		if parts, _ := filepath.Glob(filepath.Join(outDir, "*.part*")); len(parts) > 0 {
			t.Fatalf("取消后残留临时文件: %v", parts)
		}
	})
}
