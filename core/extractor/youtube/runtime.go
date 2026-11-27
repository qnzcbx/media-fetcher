package youtube

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// ErrYTDLPNotFound 表示未找到可用的 yt-dlp 解析组件。
	ErrYTDLPNotFound = errors.New("youtube: 未找到解析组件 yt-dlp，请将官方 yt-dlp 放入应用工具目录或系统 PATH，或设置 MEDIA_FETCHER_YTDLP_PATH")
	// ErrJSRuntimeNotFound 表示未找到兼容的 JavaScript 运行时（QuickJS-NG / Node >= 22 / Deno >= 2.3）。
	ErrJSRuntimeNotFound = errors.New("youtube: 未找到兼容的 JavaScript 运行时（支持 QuickJS-NG qjs、Node >= 22 或 Deno >= 2.3），请配置至应用工具目录或系统 PATH")

	semverRE = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)
)

// Config 配置 YouTube 外部解析工具的可选路径与缓存目录。
type Config struct {
	YTDLPPath     string   // 显式指定的 yt-dlp 可执行文件路径
	JSRuntime     string   // 显式指定的 JS 运行时类型（quickjs / node / deno）
	JSRuntimePath string   // 显式指定的 JS 运行时可执行文件路径
	CacheDir      string   // yt-dlp 缓存目录（为空时使用应用数据目录）
	ExtraToolDirs []string // 额外工具查找目录（用于测试或自定义部署）
}

// JSRuntimeInfo 描述经版本核验可用的外部 JavaScript 运行时。
type JSRuntimeInfo struct {
	Kind    string // "quickjs" | "node" | "deno"
	Path    string
	Version string
}

// RuntimeEnv 保存一次解析定位到的受信任工具路径。
type RuntimeEnv struct {
	YTDLPPath string
	JS        JSRuntimeInfo
	CacheDir  string
}

func exeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func appDataRoot() string {
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "media-fetcher")
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".media-fetcher")
	}
	return ""
}

func trustedToolDirs(extra []string) []string {
	var dirs []string
	seen := map[string]bool{}
	add := func(d string) {
		if d == "" {
			return
		}
		clean := filepath.Clean(d)
		key := strings.ToLower(clean)
		if !seen[key] {
			seen[key] = true
			dirs = append(dirs, clean)
		}
	}

	// 1. 应用自带的可信工具目录（与主程序相邻的 tools / bin 目录）
	if exe, err := os.Executable(); err == nil && exe != "" {
		exeDir := filepath.Dir(exe)
		add(filepath.Join(exeDir, "tools"))
		add(filepath.Join(exeDir, "bin"))
		add(exeDir)
	}
	// 2. 应用专用工具目录
	if root := appDataRoot(); root != "" {
		add(filepath.Join(root, "tools"))
	}
	// 3. 调用方/测试指定的额外受信任目录
	for _, d := range extra {
		add(d)
	}
	return dirs
}

func isRegularFile(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ResolveRuntime 按优先级发现并核验 yt-dlp 与受支持的 JS 运行时，不静默下载任何外部二进制。
func ResolveRuntime(ctx context.Context, cfg Config) (RuntimeEnv, error) {
	ytdlpPath, err := findYTDLP(cfg)
	if err != nil {
		return RuntimeEnv{}, err
	}
	jsInfo, err := findJSRuntime(ctx, cfg)
	if err != nil {
		return RuntimeEnv{}, err
	}
	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		if root := appDataRoot(); root != "" {
			cacheDir = filepath.Join(root, "cache", "yt-dlp")
		}
	}
	return RuntimeEnv{
		YTDLPPath: ytdlpPath,
		JS:        jsInfo,
		CacheDir:  cacheDir,
	}, nil
}

func findYTDLP(cfg Config) (string, error) {
	names := []string{exeName("yt-dlp")}
	if runtime.GOOS == "windows" {
		// 同时兼容目录解压版 yt-dlp_win 下的可执行文件
		names = append(names, filepath.Join("yt-dlp", "yt-dlp.exe"))
	}
	// 1 & 2. 应用自带与专用工具目录
	for _, dir := range trustedToolDirs(cfg.ExtraToolDirs) {
		for _, name := range names {
			candidate := filepath.Join(dir, name)
			if isRegularFile(candidate) {
				return candidate, nil
			}
		}
	}
	// 3. 用户显式配置路径或环境变量
	for _, explicit := range []string{cfg.YTDLPPath, os.Getenv("MEDIA_FETCHER_YTDLP_PATH")} {
		if explicit = strings.TrimSpace(explicit); explicit != "" && isRegularFile(explicit) {
			return explicit, nil
		}
	}
	// 4. 系统 PATH
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p, nil
	}
	return "", ErrYTDLPNotFound
}

func findJSRuntime(ctx context.Context, cfg Config) (JSRuntimeInfo, error) {
	type candidate struct {
		kind string
		path string
	}
	var candidates []candidate
	seen := map[string]bool{}
	add := func(kind, p string) {
		if p == "" {
			return
		}
		key := kind + ":" + strings.ToLower(filepath.Clean(p))
		if !seen[key] {
			seen[key] = true
			candidates = append(candidates, candidate{kind: kind, path: p})
		}
	}

	// 1 & 2. 应用自带与专用工具目录优先选择 JIT 运行时（Node / Deno，实测解挑战显著快于 QuickJS），其次 QuickJS-NG
	for _, dir := range trustedToolDirs(cfg.ExtraToolDirs) {
		for _, item := range []struct{ kind, bin string }{
			{"node", exeName("node")},
			{"deno", exeName("deno")},
			{"quickjs", exeName("qjs")},
		} {
			p := filepath.Join(dir, item.bin)
			if isRegularFile(p) {
				add(item.kind, p)
			}
		}
	}

	// 3. 用户显式配置路径或环境变量
	explicitPath := strings.TrimSpace(cfg.JSRuntimePath)
	if explicitPath == "" {
		explicitPath = strings.TrimSpace(os.Getenv("MEDIA_FETCHER_JS_RUNTIME_PATH"))
	}
	explicitKind := strings.ToLower(strings.TrimSpace(cfg.JSRuntime))
	if explicitKind == "" {
		explicitKind = strings.ToLower(strings.TrimSpace(os.Getenv("MEDIA_FETCHER_JS_RUNTIME")))
	}
	if explicitPath != "" && isRegularFile(explicitPath) {
		if explicitKind == "" {
			explicitKind = inferJSRuntimeKind(explicitPath)
		}
		if explicitKind != "" {
			add(explicitKind, explicitPath)
		}
	}

	// 4. 系统 PATH 中的兼容运行时（优先 Node / Deno JIT 运行时）
	for _, item := range []struct{ kind, bin string }{
		{"node", "node"},
		{"deno", "deno"},
		{"quickjs", "qjs"},
	} {
		if p, err := exec.LookPath(item.bin); err == nil {
			add(item.kind, p)
		}
	}

	for _, c := range candidates {
		if ver, ok := verifyJSRuntimeVersion(ctx, c.kind, c.path); ok {
			return JSRuntimeInfo{Kind: c.kind, Path: c.path, Version: ver}, nil
		}
	}

	return JSRuntimeInfo{}, ErrJSRuntimeNotFound
}

func inferJSRuntimeKind(path string) string {
	base := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasPrefix(base, "qjs"):
		return "quickjs"
	case strings.HasPrefix(base, "deno"):
		return "deno"
	case strings.HasPrefix(base, "node"):
		return "node"
	default:
		return ""
	}
}

type jsVersionCacheEntry struct {
	modTimeUnix int64
	size        int64
	version     string
	ok          bool
}

var (
	jsVersionCacheMu sync.Mutex
	jsVersionCache   = map[string]jsVersionCacheEntry{}
)

func verifyJSRuntimeVersion(ctx context.Context, kind, binPath string) (string, bool) {
	clean := filepath.Clean(binPath)
	cacheKey := kind + ":" + strings.ToLower(clean)
	if info, err := os.Stat(clean); err == nil && !info.IsDir() {
		jsVersionCacheMu.Lock()
		if cached, hit := jsVersionCache[cacheKey]; hit && cached.modTimeUnix == info.ModTime().UnixNano() && cached.size == info.Size() {
			jsVersionCacheMu.Unlock()
			return cached.version, cached.ok
		}
		jsVersionCacheMu.Unlock()

		vCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		cmd := exec.CommandContext(vCtx, binPath, "--version")
		hideWindow(cmd)
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return "", false
		}
		ver, ok := parseAndCheckJSVersion(kind, string(out))
		jsVersionCacheMu.Lock()
		jsVersionCache[cacheKey] = jsVersionCacheEntry{
			modTimeUnix: info.ModTime().UnixNano(),
			size:        info.Size(),
			version:     ver,
			ok:          ok,
		}
		jsVersionCacheMu.Unlock()
		return ver, ok
	}

	vCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(vCtx, binPath, "--version")
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", false
	}
	return parseAndCheckJSVersion(kind, string(out))
}

func parseAndCheckJSVersion(kind, rawOutput string) (string, bool) {
	text := strings.TrimSpace(rawOutput)
	if text == "" {
		return "", false
	}
	switch kind {
	case "quickjs":
		// QuickJS-NG 输出如 "0.18.0" 或 "quickjs-ng version 0.18.0"；原版 QuickJS 输出日期如 "2025-04-26"
		if m := semverRE.FindStringSubmatch(text); len(m) >= 3 {
			major, _ := strconv.Atoi(m[1])
			minor, _ := strconv.Atoi(m[2])
			// QuickJS-NG >= 0.12.0 具备 EJS 性能优化
			if major > 0 || minor >= 12 {
				return m[0], true
			}
			return "", false
		}
		if strings.Contains(text, "2025-") || strings.Contains(text, "2026-") || strings.Contains(text, "2027-") {
			return text, true
		}
		return "", false
	case "node":
		// yt-dlp EJS 要求 Node >= 22.0.0
		if m := semverRE.FindStringSubmatch(text); len(m) >= 3 {
			major, _ := strconv.Atoi(m[1])
			if major >= 22 {
				return m[0], true
			}
		}
		return "", false
	case "deno":
		// yt-dlp EJS 要求 Deno >= 2.3.0
		if m := semverRE.FindStringSubmatch(text); len(m) >= 3 {
			major, _ := strconv.Atoi(m[1])
			minor, _ := strconv.Atoi(m[2])
			if major > 2 || (major == 2 && minor >= 3) {
				return m[0], true
			}
		}
		return "", false
	default:
		return "", false
	}
}
