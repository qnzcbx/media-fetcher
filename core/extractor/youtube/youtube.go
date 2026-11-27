package youtube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"media-fetcher/core/extractor"
)

const (
	maxStdoutBytes = 32 << 20 // 32 MiB，足够承载超多轨道的 yt-dlp JSON 元数据
	maxStderrBytes = 256 << 10
)

var sensitiveURLRE = regexp.MustCompile(`https?://[^\s"'<>]+`)

// Extractor 通过受控外部 yt-dlp + 受支持 JS 运行时实现 YouTube 单视频解析。
// 只负责元数据与轨道发现，不承担媒体下载或 FFmpeg 封装。
type Extractor struct {
	cfg Config
}

// New 创建 YouTube Extractor 实例。
func New(cfgs ...Config) *Extractor {
	var cfg Config
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	return &Extractor{cfg: cfg}
}

// SiteID 返回平台唯一标识 "youtube"。
func (e *Extractor) SiteID() string {
	return "youtube"
}

// Match 判断输入文本或 URL 是否为首阶段支持的 YouTube 单视频或 Shorts 链接。
func (e *Extractor) Match(rawURL string) bool {
	_, err := ParseTarget(rawURL)
	return err == nil
}

// Extract 调用本地受信任 yt-dlp 解析单视频元数据及实际可用的 HTTPS 独立音视频轨道。
func (e *Extractor) Extract(ctx context.Context, rawURL string, opt extractor.ExtractOptions) (*extractor.MediaInfo, error) {
	if opt.RequireAuth || strings.TrimSpace(opt.AccountID) != "" {
		return nil, fmt.Errorf("%w: 首阶段暂不支持 YouTube 登录态解析，请使用匿名模式", extractor.ErrCredentialExpired)
	}

	target, err := ParseTarget(rawURL)
	if err != nil {
		return nil, err
	}

	env, err := ResolveRuntime(ctx, e.cfg)
	if err != nil {
		return nil, err
	}

	args := buildYTDLPArgs(env, target.CanonicalURL)
	if env.CacheDir != "" {
		_ = os.MkdirAll(env.CacheDir, 0755)
	}

	stdout, stderr, runErr := runSubprocess(ctx, env.YTDLPPath, args)
	if runErr != nil {
		return nil, classifySubprocessError(ctx, runErr, stderr)
	}
	if len(bytes.TrimSpace(stdout)) == 0 {
		return nil, classifySubprocessError(ctx, errors.New("empty json output"), stderr)
	}

	return ParseYTDLPJSON(stdout, target)
}

func buildYTDLPArgs(env RuntimeEnv, canonicalURL string) []string {
	args := []string{
		"--ignore-config",
		"--no-config-locations",
		"--no-cookies",
		"--no-cookies-from-browser",
		"--no-playlist",
		"--skip-download",
		"--dump-single-json",
		"--no-js-runtimes",
		"--js-runtimes", fmt.Sprintf("%s:%s", env.JS.Kind, env.JS.Path),
		"--extractor-args", "youtube:skip=hls,dash",
	}
	if env.CacheDir != "" {
		args = append(args, "--cache-dir", env.CacheDir)
	}
	args = append(args, "--", canonicalURL)
	return args
}

type limitedBuffer struct {
	buf      bytes.Buffer
	limit    int64
	overflow bool
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	remain := l.limit - int64(l.buf.Len())
	if remain <= 0 {
		l.overflow = true
		return len(p), nil
	}
	if int64(len(p)) > remain {
		l.overflow = true
		_, _ = l.buf.Write(p[:remain])
		return len(p), nil
	}
	return l.buf.Write(p)
}

func runSubprocess(ctx context.Context, bin string, args []string) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	hideWindow(cmd)
	// 防止外部工具读取环境变量中意外注入的敏感代理/配置之外，保留必要系统环境以便 TLS 根证书和网络代理正常工作。
	stdout := &limitedBuffer{limit: maxStdoutBytes}
	stderr := &limitedBuffer{limit: maxStderrBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = nil

	err := cmd.Run()
	if stdout.overflow {
		return nil, sanitizeStderr(stderr.buf.String()), fmt.Errorf("%w: yt-dlp 输出超出安全大小限制", extractor.ErrAPIDenied)
	}
	return stdout.buf.Bytes(), sanitizeStderr(stderr.buf.String()), err
}

// sanitizeStderr 脱敏 stderr 中的完整签名 URL，仅保留诊断摘要。
func sanitizeStderr(raw string) string {
	cleaned := sensitiveURLRE.ReplaceAllString(raw, "<redacted-url>")
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) > 1000 {
		cleaned = cleaned[:1000]
	}
	return cleaned
}

func classifySubprocessError(ctx context.Context, runErr error, stderr string) error {
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("%w: YouTube 解析超时，请检查网络或重试", extractor.ErrNetworkFailed)
		}
		return ctx.Err()
	}

	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "sign in to confirm your age") || strings.Contains(lower, "age-restricted") || strings.Contains(lower, "inappropriate for some users"):
		return fmt.Errorf("%w: 该视频受年龄限制，首阶段暂不支持登录解析", extractor.ErrAPIDenied)
	case strings.Contains(lower, "not a bot") || strings.Contains(lower, "http error 429") || strings.Contains(lower, "too many requests"):
		return fmt.Errorf("%w: YouTube 触发访问频率限制或人机校验，请稍后重试", extractor.ErrAPIDenied)
	case strings.Contains(lower, "private video"):
		return fmt.Errorf("%w: 该视频为私有视频，需要登录权限", extractor.ErrAPIDenied)
	case strings.Contains(lower, "video unavailable") || strings.Contains(lower, "this video is unavailable") || strings.Contains(lower, "this video has been removed") || strings.Contains(lower, "been terminated") || strings.Contains(lower, "does not exist"):
		return fmt.Errorf("%w: 该视频已删除或不可用", extractor.ErrAPIDenied)
	case strings.Contains(lower, "not made this video available in your country") || strings.Contains(lower, "geo-restricted") || strings.Contains(lower, "blocked it in your country"):
		return fmt.Errorf("%w: 该视频存在地区访问限制", extractor.ErrAPIDenied)
	case strings.Contains(lower, "members-only") || strings.Contains(lower, "join this channel") || strings.Contains(lower, "requires payment"):
		return fmt.Errorf("%w: 该视频为频道会员或付费专属内容", extractor.ErrAPIDenied)
	case strings.Contains(lower, "drm"):
		return fmt.Errorf("%w: 该视频受 DRM 保护，不支持下载", extractor.ErrUnsupportedMedia)
	case strings.Contains(lower, "live event will begin") || strings.Contains(lower, "premieres in") || strings.Contains(lower, "is live"):
		return fmt.Errorf("%w: 暂不支持直播流或待开播内容", extractor.ErrUnsupportedMedia)
	case strings.Contains(lower, "no supported javascript runtime") || strings.Contains(lower, "challenge solving failed"):
		return fmt.Errorf("%w: 缺少兼容的 JavaScript 运行时或挑战求解失败", ErrJSRuntimeNotFound)
	case strings.Contains(lower, "unable to download") || strings.Contains(lower, "timed out") || strings.Contains(lower, "connection reset") || strings.Contains(lower, "eof occurred") || strings.Contains(lower, "getaddrinfo failed"):
		return fmt.Errorf("%w: 访问 YouTube 网络异常，请重试", extractor.ErrNetworkFailed)
	}

	if errors.Is(runErr, io.EOF) {
		return fmt.Errorf("%w: 解析进程提前退出", extractor.ErrAPIDenied)
	}
	if stderr != "" {
		// 取最后一条 ERROR 行作为脱敏摘要
		for _, line := range strings.Split(stderr, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "ERROR:") {
				return fmt.Errorf("%w: %s", extractor.ErrAPIDenied, strings.TrimSpace(line))
			}
		}
	}
	return fmt.Errorf("%w: YouTube 解析失败: %v", extractor.ErrAPIDenied, runErr)
}
