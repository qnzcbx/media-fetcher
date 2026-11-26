package desktop

import (
	"context"
	"net/http"
	"time"
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
func probeTrackSize(ctx context.Context, client *http.Client, url string) int64 {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("Referer", "https://www.bilibili.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Encoding") != "" {
		return 0
	}
	return resp.ContentLength
}
