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
	"strconv"
	"strings"
	"time"

	"media-fetcher/pkg/fsutil"
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
)

// ProgressCallback 汇报字节下载进度的回调函数。
type ProgressCallback func(downloaded, total int64)

// DownloadOptions 配置下载的目标路径、头部及分块区间。
type DownloadOptions struct {
	OutputDir   string
	Filename    string
	Referer     string
	UserAgent   string
	EnableRange bool
	RangeStart  int64
	HasRangeEnd bool
	RangeEnd    int64
	OnProgress  ProgressCallback
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

// Downloader 负责高可靠安全流式下载，支持独占临时文件、严格 Range 校验与断电原子发布。
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
		cr := resp.Header.Get("Content-Range")
		if cr == "" {
			return "", fmt.Errorf("%w: 缺少 Content-Range 响应头", ErrInvalidContentRange)
		}
		matches := contentRangePattern.FindStringSubmatch(cr)
		if len(matches) < 3 {
			return "", fmt.Errorf("%w: 畸形的 Content-Range 头部 %q", ErrInvalidContentRange, cr)
		}
		start, _ := strconv.ParseInt(matches[1], 10, 64)
		end, _ := strconv.ParseInt(matches[2], 10, 64)

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

	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("downloader: 刷盘失败: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("downloader: 关闭文件失败: %w", err)
	}

	// 检查 Context 是否在写入完成但发布前被取消
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// 原子替换发布为正式文件
	if err := fsutil.PublishFile(partPath, finalPath, fsutil.StrategyReplace); err != nil {
		return "", fmt.Errorf("downloader: 原子发布最终文件失败: %w", err)
	}

	success = true
	return finalPath, nil
}
