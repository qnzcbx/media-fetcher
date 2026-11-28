package youtube

import (
	"context"
	"errors"
	"fmt"

	coreruntime "media-fetcher/core/runtime"
)

var (
	// ErrYTDLPNotFound 表示未找到可用的 yt-dlp 解析组件。
	ErrYTDLPNotFound = errors.New("youtube: 未找到解析组件 yt-dlp，请将官方 yt-dlp 放入应用工具目录或系统 PATH，或设置 MEDIA_FETCHER_YTDLP_PATH")
	// ErrJSRuntimeNotFound 表示未找到兼容的 JavaScript 运行时（Deno >= 2.3 / Node >= 22 / QuickJS-NG >= 0.12）。
	ErrJSRuntimeNotFound = errors.New("youtube: 未找到兼容的 JavaScript 运行时（支持 Deno >= 2.3、Node >= 22 或 QuickJS-NG >= 0.12），请配置至应用工具目录或系统 PATH")
)

// Config 配置 YouTube 外部解析工具的可选路径与缓存目录。
type Config struct {
	YTDLPPath     string   // 显式指定的 yt-dlp 可执行文件路径
	JSRuntime     string   // 显式指定的 JS 运行时类型（deno / node / quickjs）
	JSRuntimePath string   // 显式指定的 JS 运行时可执行文件路径
	CacheDir      string   // yt-dlp 缓存目录（为空时使用应用数据目录）
	ExtraToolDirs []string // 额外工具查找目录（用于测试或自定义部署）
}

// JSRuntimeInfo 描述经版本核验可用的外部 JavaScript 运行时。
type JSRuntimeInfo struct {
	Kind    string // "deno" | "node" | "quickjs"
	Path    string
	Version string
	Source  coreruntime.Source
}

// RuntimeEnv 保存一次解析定位到的受信任工具路径。
type RuntimeEnv struct {
	YTDLPPath   string
	YTDLPSource coreruntime.Source
	JS          JSRuntimeInfo
	CacheDir    string
}

// ResolveRuntime 复用共享 core/runtime 发现并核验 yt-dlp 与受支持的 JS 运行时，不静默下载任何外部二进制。
// 显式覆盖路径无效时直接报错，不静默切换其他工具。
func ResolveRuntime(ctx context.Context, cfg Config) (RuntimeEnv, error) {
	tools, err := coreruntime.ResolveYouTubeTools(ctx, coreruntime.Options{
		YTDLPPath:     cfg.YTDLPPath,
		JSRuntime:     cfg.JSRuntime,
		JSRuntimePath: cfg.JSRuntimePath,
		CacheDir:      cfg.CacheDir,
		ExtraToolDirs: cfg.ExtraToolDirs,
	})
	if err != nil {
		if errors.Is(err, coreruntime.ErrInvalidExplicitPath) {
			return RuntimeEnv{}, err
		}
		if errors.Is(err, coreruntime.ErrYTDLPNotFound) {
			return RuntimeEnv{}, fmt.Errorf("%w", ErrYTDLPNotFound)
		}
		if errors.Is(err, coreruntime.ErrJSRuntimeNotFound) {
			return RuntimeEnv{}, fmt.Errorf("%w", ErrJSRuntimeNotFound)
		}
		return RuntimeEnv{}, err
	}
	return RuntimeEnv{
		YTDLPPath:   tools.YTDLP.Path,
		YTDLPSource: tools.YTDLP.Source,
		JS: JSRuntimeInfo{
			Kind:    tools.JS.Kind,
			Path:    tools.JS.Path,
			Version: tools.JS.Version,
			Source:  tools.JS.Source,
		},
		CacheDir: tools.CacheDir,
	}, nil
}

func parseAndCheckJSVersion(kind, rawOutput string) (string, bool) {
	return coreruntime.ParseAndCheckJSVersion(kind, rawOutput)
}
