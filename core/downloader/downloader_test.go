package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDownloader_Success(t *testing.T) {
	data := []byte("hello media streaming downloader content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	d := New(nil)
	outDir := t.TempDir()

	var reportedTotal, reportedDownloaded int64
	opts := DownloadOptions{
		OutputDir: outDir,
		Filename:  "test_file.mp4",
		OnProgress: func(downloaded, total int64) {
			reportedDownloaded = downloaded
			reportedTotal = total
		},
	}

	savedPath, err := d.Download(context.Background(), nil, server.URL, opts)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	content, err := os.ReadFile(savedPath)
	if err != nil || string(content) != string(data) {
		t.Fatalf("file content mismatch: %v", err)
	}
	if reportedDownloaded != int64(len(data)) || reportedTotal != int64(len(data)) {
		t.Errorf("progress mismatch: downloaded=%d, total=%d", reportedDownloaded, reportedTotal)
	}

	// Verify .part file does not exist
	if _, err := os.Stat(savedPath + ".part"); !os.IsNotExist(err) {
		t.Error(".part file should have been cleaned up after completion")
	}
}

func TestDownloader_PathTraversalDefense(t *testing.T) {
	d := New(nil)
	outDir := t.TempDir()

	traversalFilenames := []string{
		"../../evil.exe",
		`..\..\windows\system32\malicious.dll`,
		"sub/../../hacked",
	}

	for _, fname := range traversalFilenames {
		opts := DownloadOptions{
			OutputDir: outDir,
			Filename:  fname,
		}
		// Either sanitized to stay inside outDir or rejected with ErrPathTraversal
		savedPath, err := d.Download(context.Background(), nil, "https://example.com/fake", opts)
		if err == nil {
			// If it succeeded, verify it stayed strictly within outDir
			rel, _ := filepath.Rel(outDir, savedPath)
			if filepath.IsAbs(rel) || len(rel) >= 2 && rel[:2] == ".." {
				t.Fatalf("path traversal escaped outDir for %s: %s", fname, savedPath)
			}
		}
	}
}

func TestDownloader_CancellationCleansUpPart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		w.WriteHeader(http.StatusOK)
		// Write partial chunk and pause
		_, _ = w.Write([]byte("start chunk"))
		w.(http.Flusher).Flush()
		time.Sleep(500 * time.Millisecond)
	}))
	defer server.Close()

	d := New(nil)
	outDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	opts := DownloadOptions{
		OutputDir: outDir,
		Filename:  "cancel_test.bin",
	}

	_, err := d.Download(ctx, nil, server.URL, opts)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}

	targetFile := filepath.Join(outDir, "cancel_test.bin")
	partFile := targetFile + ".part"

	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Error("final file should NOT exist after canceled download")
	}
	if _, err := os.Stat(partFile); !os.IsNotExist(err) {
		t.Error("part file should have been removed on cancellation")
	}
}

func TestDownloader_RangeDownload(t *testing.T) {
	fullData := []byte("0123456789abcdefghij")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "bytes=5-9" {
			http.Error(w, "invalid range header", 400)
			return
		}
		w.Header().Set("Content-Range", "bytes 5-9/20")
		w.Header().Set("Content-Length", "5")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(fullData[5:10])
	}))
	defer server.Close()

	d := New(nil)
	outDir := t.TempDir()
	opts := DownloadOptions{
		OutputDir:  outDir,
		Filename:   "range_chunk.bin",
		RangeStart: 5,
		RangeEnd:   9,
	}

	savedPath, err := d.Download(context.Background(), nil, server.URL, opts)
	if err != nil {
		t.Fatalf("range download failed: %v", err)
	}

	content, err := os.ReadFile(savedPath)
	if err != nil || string(content) != "56789" {
		t.Fatalf("range content mismatch: got %q, want '56789'", string(content))
	}
}

func TestMuxVideoAudio_RealFFmpeg(t *testing.T) {
	videoM4s := filepath.Join("..", "..", "references", "lofter_test", "bill-downloads", "video.m4s")
	audioM4s := filepath.Join("..", "..", "references", "lofter_test", "bill-downloads", "audio.m4s")

	if _, err := os.Stat(videoM4s); os.IsNotExist(err) {
		t.Skip("skipping ffmpeg mux test: sample video.m4s not found")
	}
	if _, err := os.Stat(audioM4s); os.IsNotExist(err) {
		t.Skip("skipping ffmpeg mux test: sample audio.m4s not found")
	}

	ffmpegPath, err := FindFFmpeg()
	if err != nil {
		t.Skipf("skipping ffmpeg mux test: %v", err)
	}

	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "test_muxed.mp4")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = MuxVideoAudio(ctx, ffmpegPath, videoM4s, audioM4s, outFile)
	if err != nil {
		t.Fatalf("MuxVideoAudio failed: %v", err)
	}

	stat, err := os.Stat(outFile)
	if err != nil || stat.Size() == 0 {
		t.Fatalf("muxed mp4 is missing or empty, size: %v, err: %v", stat.Size(), err)
	}
}
