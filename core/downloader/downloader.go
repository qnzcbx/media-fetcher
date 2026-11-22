package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	ErrPathTraversal       = errors.New("downloader: path traversal detected")
	ErrInvalidHTTPStatus   = errors.New("downloader: unexpected HTTP status")
	ErrInvalidContentRange = errors.New("downloader: invalid content-range header")
	invalidWindowsChars    = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
)

// ProgressCallback reports byte transfer progress.
type ProgressCallback func(downloaded, total int64)

// DownloadOptions configures destination, headers and progress hooks for a download.
type DownloadOptions struct {
	OutputDir  string
	Filename   string
	Referer    string
	UserAgent  string
	RangeStart int64
	RangeEnd   int64
	OnProgress ProgressCallback
}

// SanitizeFilename cleanses a filename to prevent Windows filesystem issues and injection.
func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = invalidWindowsChars.ReplaceAllString(name, "_")
	// Trim trailing periods and spaces (invalid on Windows)
	name = strings.TrimRight(name, ". ")
	if name == "" {
		name = "media_download"
	}
	return name
}

// Downloader executes stream downloads with temporary file safety and Range support.
type Downloader struct {
	defaultClient *http.Client
}

// New creates a new Downloader.
func New(client *http.Client) *Downloader {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &Downloader{defaultClient: client}
}

// Download downloads content from mediaURL directly into a streaming file.
// It uses .part temporary files, guarantees no corrupt files left on failure,
// and enforces path traversal prevention.
func (d *Downloader) Download(ctx context.Context, client *http.Client, mediaURL string, opts DownloadOptions) (string, error) {
	if client == nil {
		client = d.defaultClient
	}

	cleanOutputDir, err := filepath.Abs(opts.OutputDir)
	if err != nil {
		return "", fmt.Errorf("downloader: invalid output dir: %w", err)
	}

	safeName := SanitizeFilename(opts.Filename)
	finalPath := filepath.Clean(filepath.Join(cleanOutputDir, safeName))

	// Enforce Path Traversal Defense
	rel, err := filepath.Rel(cleanOutputDir, finalPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", ErrPathTraversal
	}

	if err := os.MkdirAll(cleanOutputDir, 0755); err != nil {
		return "", fmt.Errorf("downloader: create output dir failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return "", err
	}

	ua := opts.UserAgent
	if ua == "" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}
	req.Header.Set("User-Agent", ua)
	if opts.Referer != "" {
		req.Header.Set("Referer", opts.Referer)
	}

	isRange := false
	if opts.RangeStart > 0 || opts.RangeEnd > 0 {
		isRange = true
		if opts.RangeEnd > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", opts.RangeStart, opts.RangeEnd))
		} else {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", opts.RangeStart))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloader: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if isRange {
		if resp.StatusCode != http.StatusPartialContent {
			return "", fmt.Errorf("%w: expected 206 Partial Content, got %d", ErrInvalidHTTPStatus, resp.StatusCode)
		}
		// Validate Content-Range header
		cr := resp.Header.Get("Content-Range")
		if cr == "" || !strings.HasPrefix(cr, "bytes ") {
			return "", fmt.Errorf("%w: missing or malformed Content-Range %q", ErrInvalidContentRange, cr)
		}
	} else {
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("%w: expected 200 OK, got %d", ErrInvalidHTTPStatus, resp.StatusCode)
		}
	}

	totalSize := resp.ContentLength

	// Temporary file setup
	partPath := finalPath + ".part"
	file, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("downloader: create temp file failed: %w", err)
	}

	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(partPath)
		}
	}()

	buf := make([]byte, 64*1024)
	var downloaded int64

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := file.Write(buf[:n]); wErr != nil {
				return "", fmt.Errorf("downloader: write error: %w", wErr)
			}
			downloaded += int64(n)
			if opts.OnProgress != nil {
				opts.OnProgress(downloaded, totalSize)
			}
		}
		if rErr != nil {
			if errors.Is(rErr, io.EOF) {
				break
			}
			return "", fmt.Errorf("downloader: stream read error: %w", rErr)
		}
	}

	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("downloader: sync failed: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("downloader: close failed: %w", err)
	}

	// Atomic rename to final output
	_ = os.Remove(finalPath)
	if err := os.Rename(partPath, finalPath); err != nil {
		return "", fmt.Errorf("downloader: finalize rename failed: %w", err)
	}

	success = true
	return finalPath, nil
}
