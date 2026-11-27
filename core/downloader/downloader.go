package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"media-fetcher/pkg/fsutil"
)

const (
	defaultUserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	defaultChunkRetries   = 3
	defaultRetryBaseDelay = 500 * time.Millisecond
	fallbackSmallChunk    = 2 << 20 // 2 MiB，仅在首块大区间 403 时做单次降级验证
)

var (
	// ErrPathTraversal 表示检测到路径穿越攻击。
	ErrPathTraversal = errors.New("downloader: 检测到非法路径穿越尝试")
	// ErrInvalidHTTPStatus 表示服务端返回了非预期的 HTTP 状态码。
	ErrInvalidHTTPStatus = errors.New("downloader: 非预期的 HTTP 响应状态码")
	// ErrInvalidContentRange 表示 Content-Range 头部缺失、格式错误或与请求区间不匹配。
	ErrInvalidContentRange = errors.New("downloader: Content-Range 头部无效或格式错误")
	// ErrRangeMismatch 表示服务端返回的区间与请求区间不符。
	ErrRangeMismatch = errors.New("downloader: Content-Range 区间与客户端请求不符")
	// ErrIncompleteDownload 表示实收字节数短于声明区间或长度，下载未完整完成。
	ErrIncompleteDownload = errors.New("downloader: 实收数据不完整或传输提前中断")

	invalidWindowsChars  = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	windowsReservedNames = regexp.MustCompile(`^(?i)(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(\..*)?$`)
	contentRangePattern  = regexp.MustCompile(`^bytes\s+(\d+)-(\d+)/(\d+|\*)$`)

	// safeRequestHeaders 限制平台轨道元数据可注入的 HTTP 请求头白名单，禁止注入 Cookie/Authorization/Host/Range 等敏感或控制头。
	safeRequestHeaders = map[string]bool{
		"User-Agent":      true,
		"Referer":         true,
		"Origin":          true,
		"Accept":          true,
		"Accept-Language": true,
		"Sec-Fetch-Mode":  true,
		"Sec-Fetch-Site":  true,
		"Sec-Fetch-Dest":  true,
	}
)

// ProgressCallback 汇报字节下载进度的回调函数。
type ProgressCallback func(downloaded, total int64)

// DownloadOptions 配置下载的目标路径、头部、分块及重试策略。
type DownloadOptions struct {
	OutputDir    string
	Filename     string
	Referer      string
	UserAgent    string
	Headers      map[string]string
	EnableRange  bool
	RangeStart   int64
	HasRangeEnd  bool
	RangeEnd     int64
	ChunkSize    int64         // >0 且未显式 EnableRange 时启用顺序 Range 分块下载
	ExpectedSize int64         // 轨道声明的已知总大小（可选，<=0 时自动探测）
	MaxRetries   int           // 分块模式下单块瞬时错误最大重试次数（<=0 默认 3）
	RetryDelay   time.Duration // 分块重试基础退避时长（<=0 默认 500ms）
	OnProgress   ProgressCallback
}

// SanitizeHeaders 过滤轨道或调用方提供的 HTTP 请求头，仅保留白名单且不含换行控制字符的安全字段。
func SanitizeHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		key := http.CanonicalHeaderKey(strings.TrimSpace(k))
		if !safeRequestHeaders[key] {
			continue
		}
		val := strings.TrimSpace(v)
		if val == "" || strings.ContainsAny(val, "\r\n") {
			continue
		}
		out[key] = val
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ApplySafeHeaders 将允许的安全请求头应用到 HTTP 请求，同时保持默认 User-Agent 与显式字段兼容。
func ApplySafeHeaders(req *http.Request, headers map[string]string, userAgent, referer string) {
	for k, v := range SanitizeHeaders(headers) {
		req.Header.Set(k, v)
	}
	if ua := strings.TrimSpace(userAgent); ua != "" && !strings.ContainsAny(ua, "\r\n") {
		req.Header.Set("User-Agent", ua)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", defaultUserAgent)
	}
	if ref := strings.TrimSpace(referer); ref != "" && !strings.ContainsAny(ref, "\r\n") {
		req.Header.Set("Referer", ref)
	}
}

// SanitizeFilename 清洗文件名，过滤 Windows 非法字符及保留设备名（CON, PRN, AUX, NUL等）。
func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = invalidWindowsChars.ReplaceAllString(name, "_")
	// 去除 Windows 末尾非法点与空格
	name = strings.TrimRight(name, ". ")
	if name == "" {
		name = "media_download"
	}
	// 防御 Windows 保留设备名称（如 CON, PRN, AUX, NUL, COM1-9, LPT1-9 及其带后缀形式）
	if windowsReservedNames.MatchString(name) {
		name = "_" + name + "_"
	}
	return name
}

// Downloader 负责高可靠安全流式下载，支持独占临时文件、严格 Range 校验、分块重试与断电原子发布。
type Downloader struct {
	defaultClient *http.Client
}

// New 创建新的 Downloader 实例。
func New(client *http.Client) *Downloader {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &Downloader{defaultClient: client}
}

func validateDownloadURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("downloader: 无效的媒体下载链接")
	}
	return nil
}

// Download 从 mediaURL 流式下载到目标文件。
// 严格防范路径穿越，使用进程专属独占临时文件，并在严格校验完整性后原子发布。
func (d *Downloader) Download(ctx context.Context, client *http.Client, mediaURL string, opts DownloadOptions) (string, error) {
	if client == nil {
		client = d.defaultClient
	}

	cleanOutputDir, err := filepath.Abs(opts.OutputDir)
	if err != nil {
		return "", fmt.Errorf("downloader: 无效的输出目录: %w", err)
	}

	safeName := SanitizeFilename(opts.Filename)
	finalPath := filepath.Clean(filepath.Join(cleanOutputDir, safeName))

	// 路径穿越防御 1：词法前缀检查
	rel, err := filepath.Rel(cleanOutputDir, finalPath)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", ErrPathTraversal
	}

	if err := os.MkdirAll(cleanOutputDir, 0755); err != nil {
		return "", fmt.Errorf("downloader: 创建输出目录失败: %w", err)
	}

	// 路径穿越防御 2：物理软链接解析校验（防止通过符号链接跳出沙盒）
	if realOutputDir, err := filepath.EvalSymlinks(cleanOutputDir); err == nil {
		if realFinalDir, err := filepath.EvalSymlinks(filepath.Dir(finalPath)); err == nil {
			if rel2, err := filepath.Rel(realOutputDir, realFinalDir); err != nil || strings.HasPrefix(rel2, "..") {
				return "", ErrPathTraversal
			}
		}
	}

	if err := validateDownloadURL(mediaURL); err != nil {
		return "", err
	}

	// 顺序 Range 分块下载模式（仅在显式配置 ChunkSize 且非单次手动 Range 时启用）
	if !opts.EnableRange && opts.ChunkSize > 0 {
		return d.downloadChunked(ctx, client, mediaURL, finalPath, opts)
	}

	return d.downloadSingle(ctx, client, mediaURL, finalPath, opts)
}

func (d *Downloader) downloadSingle(ctx context.Context, client *http.Client, mediaURL, finalPath string, opts DownloadOptions) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return "", err
	}
	ApplySafeHeaders(req, opts.Headers, opts.UserAgent, opts.Referer)

	isRange := opts.EnableRange
	if isRange {
		if opts.HasRangeEnd {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", opts.RangeStart, opts.RangeEnd))
		} else {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", opts.RangeStart))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloader: HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	var expectedLen int64 = -1

	if isRange {
		if resp.StatusCode != http.StatusPartialContent {
			return "", fmt.Errorf("%w: 请求 Range 时期望 206 Partial Content, 实际返回 %d", ErrInvalidHTTPStatus, resp.StatusCode)
		}
		start, end, _, parseErr := parseContentRange(resp.Header.Get("Content-Range"))
		if parseErr != nil {
			return "", parseErr
		}
		// 严格校验返回区间与客户端请求区间是否一致
		if start != opts.RangeStart {
			return "", fmt.Errorf("%w: 期望起始 %d, 实际返回 %d", ErrRangeMismatch, opts.RangeStart, start)
		}
		if opts.HasRangeEnd && end != opts.RangeEnd {
			return "", fmt.Errorf("%w: 期望结束 %d, 实际返回 %d", ErrRangeMismatch, opts.RangeEnd, end)
		}
		expectedLen = end - start + 1
		if resp.ContentLength >= 0 && resp.ContentLength != expectedLen {
			return "", fmt.Errorf("%w: 返回 Content-Length %d 与 Range 长度 %d 不符", ErrInvalidContentRange, resp.ContentLength, expectedLen)
		}
	} else {
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("%w: 期望 200 OK, 实际返回 %d", ErrInvalidHTTPStatus, resp.StatusCode)
		}
		if resp.ContentLength > 0 {
			expectedLen = resp.ContentLength
		}
	}

	// 独占临时文件：包含 PID 与纳秒时间戳，使用 O_EXCL 打开，避免并发同名任务相互踩踏覆写
	partPath := fmt.Sprintf("%s.part.%d.%d", finalPath, os.Getpid(), time.Now().UnixNano())
	file, err := os.OpenFile(partPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("downloader: 创建独占临时文件失败: %w", err)
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
				return "", fmt.Errorf("downloader: 写入磁盘失败: %w", wErr)
			}
			downloaded += int64(n)
			if opts.OnProgress != nil {
				opts.OnProgress(downloaded, expectedLen)
			}
		}
		if rErr != nil {
			if errors.Is(rErr, io.EOF) {
				break
			}
			if errors.Is(rErr, io.ErrUnexpectedEOF) {
				return "", fmt.Errorf("%w: %v", ErrIncompleteDownload, rErr)
			}
			return "", fmt.Errorf("downloader: 流读取中断: %w", rErr)
		}
	}

	// 完整性校验：实收长度必须等于声明长度，严禁发布残缺文件
	if expectedLen >= 0 && downloaded != expectedLen {
		return "", fmt.Errorf("%w: 期望 %d 字节, 实收 %d 字节", ErrIncompleteDownload, expectedLen, downloaded)
	}

	if err := finalizePartFile(ctx, file, partPath, finalPath); err != nil {
		return "", err
	}

	success = true
	return finalPath, nil
}

func (d *Downloader) downloadChunked(ctx context.Context, client *http.Client, mediaURL, finalPath string, opts DownloadOptions) (string, error) {
	maxRetries := opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultChunkRetries
	}
	baseDelay := opts.RetryDelay
	if baseDelay <= 0 {
		baseDelay = defaultRetryBaseDelay
	}

	totalSize := opts.ExpectedSize
	if totalSize <= 0 {
		probed, err := probeTotalByRange(ctx, client, mediaURL, opts.Headers, opts.UserAgent, opts.Referer)
		if err != nil {
			return "", err
		}
		totalSize = probed
	}
	if totalSize <= 0 {
		return "", fmt.Errorf("%w: 分块下载无法确定有效文件总大小", ErrInvalidContentRange)
	}

	partPath := fmt.Sprintf("%s.part.%d.%d", finalPath, os.Getpid(), time.Now().UnixNano())
	file, err := os.OpenFile(partPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("downloader: 创建独占临时文件失败: %w", err)
	}

	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(partPath)
		}
	}()

	chunkSize := opts.ChunkSize
	var offset int64
	var reportedHighWater int64
	reducedChunkFor403 := false
	buf := make([]byte, 64*1024)

	for offset < totalSize {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		end := offset + chunkSize - 1
		if end >= totalSize {
			end = totalSize - 1
		}
		expectedLen := end - offset + 1

		var chunkErr error
		for attempt := 0; attempt <= maxRetries; attempt++ {
			if err := ctx.Err(); err != nil {
				return "", err
			}

			var retryable bool
			var status403 bool
			chunkErr, retryable, status403 = fetchAndWriteChunk(
				ctx, client, mediaURL, opts, file, buf,
				offset, end, expectedLen, totalSize,
				&reportedHighWater,
			)
			if chunkErr == nil {
				break
			}
			// 仅当首块且分块大于 2 MiB 时收到 403，尝试单次缩小分块以兼容严格分块上限的边缘节点；
			// 若缩小后仍 403 则立即失败，绝不掩盖真实权限/失效错误。
			if status403 && offset == 0 && chunkSize > fallbackSmallChunk && !reducedChunkFor403 {
				reducedChunkFor403 = true
				chunkSize = fallbackSmallChunk
				end = offset + chunkSize - 1
				if end >= totalSize {
					end = totalSize - 1
				}
				expectedLen = end - offset + 1
				attempt = -1
				continue
			}
			if !retryable || attempt >= maxRetries {
				return "", chunkErr
			}

			delay := baseDelay << min(attempt, 4)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}

		offset += expectedLen
	}

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("downloader: 校验分块文件大小失败: %w", err)
	}
	if info.Size() != totalSize {
		return "", fmt.Errorf("%w: 分块汇总大小不符，期望 %d 字节, 实际 %d 字节", ErrIncompleteDownload, totalSize, info.Size())
	}

	if err := finalizePartFile(ctx, file, partPath, finalPath); err != nil {
		return "", err
	}

	success = true
	return finalPath, nil
}

func fetchAndWriteChunk(
	ctx context.Context,
	client *http.Client,
	mediaURL string,
	opts DownloadOptions,
	file *os.File,
	buf []byte,
	offset, end, expectedLen, totalSize int64,
	reportedHighWater *int64,
) (err error, retryable bool, is403 bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return err, false, false
	}
	ApplySafeHeaders(req, opts.Headers, opts.UserAgent, opts.Referer)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, end))

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err(), false, false
		}
		return fmt.Errorf("downloader: 分块 HTTP 请求失败: %w", err), true, false
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusPartialContent:
		// 正常 206，继续严格校验 Content-Range
	case http.StatusForbidden:
		return fmt.Errorf("%w: 请求分块返回 403 Forbidden", ErrInvalidHTTPStatus), false, true
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return fmt.Errorf("%w: 请求分块返回可重试状态码 %d", ErrInvalidHTTPStatus, resp.StatusCode), true, false
	default:
		// 含 200 OK（服务端忽略 Range）、401/404/410/416 等明确错误，不盲目重试
		return fmt.Errorf("%w: 请求分块期望 206 Partial Content, 实际返回 %d", ErrInvalidHTTPStatus, resp.StatusCode), false, false
	}

	start, crEnd, crTotal, parseErr := parseContentRange(resp.Header.Get("Content-Range"))
	if parseErr != nil {
		return parseErr, false, false
	}
	if crTotal <= 0 {
		return fmt.Errorf("%w: 分块响应缺少有效总大小", ErrInvalidContentRange), false, false
	}
	if crTotal != totalSize {
		return fmt.Errorf("%w: 服务端文件总大小已变化，期望 %d, 实际 %d", ErrRangeMismatch, totalSize, crTotal), false, false
	}
	if start != offset || crEnd != end {
		return fmt.Errorf("%w: 期望区间 %d-%d, 实际返回 %d-%d", ErrRangeMismatch, offset, end, start, crEnd), false, false
	}
	if resp.ContentLength >= 0 && resp.ContentLength != expectedLen {
		return fmt.Errorf("%w: 返回 Content-Length %d 与 Range 长度 %d 不符", ErrInvalidContentRange, resp.ContentLength, expectedLen), false, false
	}

	var chunkRead int64
	for {
		if err := ctx.Err(); err != nil {
			return err, false, false
		}

		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if chunkRead+int64(n) > expectedLen {
				return fmt.Errorf("%w: 分块实收字节超出预期区间 %d", ErrIncompleteDownload, expectedLen), false, false
			}
			// 按精确偏移写入，保证分块重试不会重复追加或错位。
			if _, wErr := file.WriteAt(buf[:n], offset+chunkRead); wErr != nil {
				return fmt.Errorf("downloader: 写入分块数据失败: %w", wErr), false, false
			}
			chunkRead += int64(n)
			if opts.OnProgress != nil {
				current := offset + chunkRead
				if current > *reportedHighWater {
					*reportedHighWater = current
				}
				opts.OnProgress(*reportedHighWater, totalSize)
			}
		}
		if rErr != nil {
			if errors.Is(rErr, io.EOF) {
				break
			}
			if ctx.Err() != nil {
				return ctx.Err(), false, false
			}
			return fmt.Errorf("%w: 分块流读取中断: %v", ErrIncompleteDownload, rErr), true, false
		}
	}

	if chunkRead != expectedLen {
		return fmt.Errorf("%w: 分块实收不足，期望 %d 字节, 实收 %d 字节", ErrIncompleteDownload, expectedLen, chunkRead), true, false
	}
	return nil, false, false
}

func probeTotalByRange(ctx context.Context, client *http.Client, mediaURL string, headers map[string]string, userAgent, referer string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return 0, err
	}
	ApplySafeHeaders(req, headers, userAgent, referer)
	req.Header.Set("Range", "bytes=0-0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("downloader: 探测文件大小失败: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))

	if resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("%w: Range 探测期望 206 Partial Content, 实际返回 %d", ErrInvalidHTTPStatus, resp.StatusCode)
	}
	start, end, total, parseErr := parseContentRange(resp.Header.Get("Content-Range"))
	if parseErr != nil {
		return 0, parseErr
	}
	if start != 0 || end != 0 || total <= 0 {
		return 0, fmt.Errorf("%w: Range 探测返回无效区间 %d-%d/%d", ErrRangeMismatch, start, end, total)
	}
	return total, nil
}

func parseContentRange(cr string) (start, end, total int64, err error) {
	if cr == "" {
		return 0, 0, 0, fmt.Errorf("%w: 缺少 Content-Range 响应头", ErrInvalidContentRange)
	}
	matches := contentRangePattern.FindStringSubmatch(cr)
	if len(matches) < 4 {
		return 0, 0, 0, fmt.Errorf("%w: 畸形的 Content-Range 头部 %q", ErrInvalidContentRange, cr)
	}
	start, err = strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("%w: 无法解析起始偏移 %q", ErrInvalidContentRange, cr)
	}
	end, err = strconv.ParseInt(matches[2], 10, 64)
	if err != nil || end < start {
		return 0, 0, 0, fmt.Errorf("%w: 非法区间 %q", ErrInvalidContentRange, cr)
	}
	if matches[3] == "*" {
		total = -1
	} else {
		total, err = strconv.ParseInt(matches[3], 10, 64)
		if err != nil || total <= end {
			return 0, 0, 0, fmt.Errorf("%w: 非法总长度 %q", ErrInvalidContentRange, cr)
		}
	}
	return start, end, total, nil
}

func finalizePartFile(ctx context.Context, file *os.File, partPath, finalPath string) error {
	if err := file.Sync(); err != nil {
		return fmt.Errorf("downloader: 刷盘失败: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("downloader: 关闭文件失败: %w", err)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := fsutil.PublishFile(partPath, finalPath, fsutil.StrategyReplace); err != nil {
		return fmt.Errorf("downloader: 原子发布最终文件失败: %w", err)
	}
	return nil
}
