package desktop

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"media-fetcher/core/downloader"
	"media-fetcher/core/extractor"
)

const (
	// defaultExtractSnapshotTTL 严格限制内存解析快照的最长复用窗口，绝不落盘持久化签名媒体 URL。
	defaultExtractSnapshotTTL = 90 * time.Second
	// minSignedURLLeadTime 若签名媒体 URL 的 expire 时间不足 2 分钟，则在发起下载前直接放弃快照并重新解析。
	minSignedURLLeadTime = 2 * time.Minute
)

type extractSnapshot struct {
	siteID       string
	canonicalURL string
	accountID    string
	media        *extractor.MediaInfo
	expiresAt    time.Time
}

func (s *DesktopService) now() time.Time {
	if s.snapshotNow != nil {
		return s.snapshotNow()
	}
	return time.Now()
}

func (s *DesktopService) extractSnapshotTTL() time.Duration {
	if s.snapshotTTL > 0 {
		return s.snapshotTTL
	}
	return defaultExtractSnapshotTTL
}

// ClearExtractSnapshot 主动清除当前内存解析快照（关闭资源选择弹窗或切换资源时调用）。
func (s *DesktopService) ClearExtractSnapshot() {
	s.snapshotMu.Lock()
	s.snapshot = nil
	s.snapshotMu.Unlock()
}

func (s *DesktopService) invalidateExtractSnapshotForAccount(siteID, accountID string) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	if s.snapshot == nil {
		return
	}
	if strings.EqualFold(s.snapshot.siteID, siteID) && (accountID == "" || s.snapshot.accountID == accountID) {
		s.snapshot = nil
	}
}

// isSnapshotEligibleSite 仅对外部子进程解析开销显著的 YouTube 启用短 TTL 内存快照，
// 保持 Bilibili（含番剧 PGC 每次重拉播放地址）与 LOFTER 原下载与重解析策略完全不变。
func isSnapshotEligibleSite(siteID string) bool {
	return strings.EqualFold(strings.TrimSpace(siteID), "youtube")
}

func (s *DesktopService) saveExtractSnapshot(rawURL, accountID string, media *extractor.MediaInfo) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	if media == nil || !isSnapshotEligibleSite(media.SiteID) || (len(media.VideoStreams) == 0 && len(media.AudioStreams) == 0) {
		s.snapshot = nil
		return
	}
	canonical := normalizeSnapshotURL(rawURL, media.SourceURL)
	if canonical == "" {
		s.snapshot = nil
		return
	}
	s.snapshot = &extractSnapshot{
		siteID:       strings.ToLower(media.SiteID),
		canonicalURL: canonical,
		accountID:    accountID,
		media:        cloneMediaInfo(media),
		expiresAt:    s.now().Add(s.extractSnapshotTTL()),
	}
}

func (s *DesktopService) lookupExtractSnapshot(siteID, rawURL, accountID string) *extractor.MediaInfo {
	if !isSnapshotEligibleSite(siteID) {
		return nil
	}
	canonical := normalizeSnapshotURL(rawURL, "")
	if canonical == "" {
		return nil
	}
	now := s.now()

	s.snapshotMu.Lock()
	snap := s.snapshot
	if snap == nil {
		s.snapshotMu.Unlock()
		return nil
	}
	if !now.Before(snap.expiresAt) {
		s.snapshot = nil
		s.snapshotMu.Unlock()
		return nil
	}
	if !strings.EqualFold(snap.siteID, siteID) || snap.canonicalURL != canonical || snap.accountID != accountID {
		s.snapshotMu.Unlock()
		return nil
	}
	cloned := cloneMediaInfo(snap.media)
	s.snapshotMu.Unlock()

	// 若绑定了登录账号，再次核对账号会话仍处于有效未撤销状态。
	if accountID != "" {
		if s.credService == nil {
			return nil
		}
		if _, err := s.credService.GetClient(siteID, accountID); err != nil {
			s.invalidateExtractSnapshotForAccount(siteID, accountID)
			return nil
		}
	}
	return cloned
}

func normalizeSnapshotURL(rawURL, fallbackSource string) string {
	if cleaned := cleanSource(rawURL); cleaned != "" {
		return cleaned
	}
	return cleanSource(fallbackSource)
}

func cloneMediaInfo(src *extractor.MediaInfo) *extractor.MediaInfo {
	if src == nil {
		return nil
	}
	cp := *src
	if len(src.Episodes) > 0 {
		cp.Episodes = append([]extractor.Episode(nil), src.Episodes...)
	}
	if len(src.VideoStreams) > 0 {
		cp.VideoStreams = make([]extractor.VideoStream, len(src.VideoStreams))
		for i, vs := range src.VideoStreams {
			if len(vs.BackupURLs) > 0 {
				vs.BackupURLs = append([]string(nil), vs.BackupURLs...)
			}
			if len(vs.Headers) > 0 {
				h := make(map[string]string, len(vs.Headers))
				for k, v := range vs.Headers {
					h[k] = v
				}
				vs.Headers = h
			}
			cp.VideoStreams[i] = vs
		}
	}
	if len(src.AudioStreams) > 0 {
		cp.AudioStreams = make([]extractor.AudioStream, len(src.AudioStreams))
		for i, as := range src.AudioStreams {
			if len(as.BackupURLs) > 0 {
				as.BackupURLs = append([]string(nil), as.BackupURLs...)
			}
			if len(as.Headers) > 0 {
				h := make(map[string]string, len(as.Headers))
				for k, v := range as.Headers {
					h[k] = v
				}
				as.Headers = h
			}
			cp.AudioStreams[i] = as
		}
	}
	if len(src.Images) > 0 {
		cp.Images = make([]extractor.ImageItem, len(src.Images))
		for i, img := range src.Images {
			if len(img.BackupURLs) > 0 {
				img.BackupURLs = append([]string(nil), img.BackupURLs...)
			}
			cp.Images[i] = img
		}
	}
	return &cp
}

// isSignedURLExpiringSoon 检查签名媒体 URL 中的 expire Unix 时间戳（如 YouTube googlevideo URL）。
func isSignedURLExpiringSoon(rawURL string, now time.Time) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	expStr := strings.TrimSpace(u.Query().Get("expire"))
	if expStr == "" {
		return false
	}
	expUnix, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || expUnix < 1_000_000_000 {
		return false
	}
	return now.Add(minSignedURLLeadTime).Unix() >= expUnix
}

// shouldRetryWithFreshExtract 仅在复用内存快照下载遭拒/失效时允许单次回退重新解析；
// 绝不把所有 403 永久视作 URL 过期——重新解析后若仍 403 则直接返回真实失败。
func shouldRetryWithFreshExtract(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, downloader.ErrInvalidHTTPStatus) ||
		errors.Is(err, downloader.ErrInvalidContentRange) ||
		errors.Is(err, downloader.ErrRangeMismatch)
}
