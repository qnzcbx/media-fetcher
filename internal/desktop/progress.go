package desktop

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"media-fetcher/core/downloader"
)

type trackProgress struct {
	downloaded [2]int64
	total      [2]int64
}

func (p *trackProgress) update(track int, downloaded, total int64) (int64, int64) {
	p.downloaded[track] = downloaded
	p.total[track] = total
	if p.total[0] <= 0 || p.total[1] <= 0 {
		return 0, 0
	}
	sum := p.total[0] + p.total[1]
	if sum <= 0 || p.downloaded[0] > p.total[0] || p.downloaded[1] > p.total[1] {
		return 0, 0
	}
	return p.downloaded[0] + p.downloaded[1], sum
}

// probeTrackSize 优先使用轨道声明的可靠大小；缺失时使用与下载一致的安全请求头探测，
// HEAD 被拒（如 403/405）时自动回退为 Range: bytes=0-0 探测。
func probeTrackSize(ctx context.Context, client *http.Client, rawURL string, knownSize int64, headers map[string]string) int64 {
	if knownSize > 0 {
		return knownSize
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	probeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0
	}
	downloader.ApplySafeHeaders(req, headers, "", "")
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK && resp.Header.Get("Content-Encoding") == "" && resp.ContentLength > 0 {
			return resp.ContentLength
		}
		if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusMethodNotAllowed {
			return 0
		}
	}

	// HEAD 不受支持或返回 403/405 时回退为 bytes=0-0 探测
	reqRange, err := http.NewRequestWithContext(probeCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0
	}
	downloader.ApplySafeHeaders(reqRange, headers, "", "")
	reqRange.Header.Set("Range", "bytes=0-0")
	respRange, err := client.Do(reqRange)
	if err != nil {
		return 0
	}
	defer respRange.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(respRange.Body, 1024))
	if respRange.StatusCode != http.StatusPartialContent {
		return 0
	}
	cr := respRange.Header.Get("Content-Range")
	if idx := strings.LastIndex(cr, "/"); idx >= 0 && idx+1 < len(cr) {
		if total, parseErr := strconv.ParseInt(cr[idx+1:], 10, 64); parseErr == nil && total > 0 {
			return total
		}
	}
	return 0
}
