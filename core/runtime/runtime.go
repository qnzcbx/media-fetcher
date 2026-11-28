package runtime

import (
	"context"
	"errors"
	"fmt"
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

// Source 标识运行组件的来源层级。
type Source string

const (
	SourceExplicit Source = "explicit" // 用户显式配置或环境变量覆盖
	SourceManaged  Source = "managed"  // 预留：未来应用内受控更新目录（当前阶段不自动更新）
	SourceBundled  Source = "bundled"  // 安装包随附 <exeDir>/tools
	SourceAppData  Source = "appdata"  // 应用专用工具目录 %APPDATA%/media-fetcher/tools
	SourceSystem   Source = "system"   // 系统 PATH 或常见安装路径
	SourceMissing  Source = "missing"  // 未找到可用组件
)

var (
	// ErrInvalidExplicitPath 表示用户显式指定的工具路径不存在、不可执行或不满足版本要求。
	ErrInvalidExplicitPath = errors.New("runtime: 显式指定的组件路径无效")
	// ErrFFmpegNotFound 表示未找到配对可用的 FFmpeg / ffprobe。
	ErrFFmpegNotFound = errors.New("runtime: 未找到可用的 FFmpeg / ffprobe")
	// ErrYTDLPNotFound 表示未找到可用的 yt-dlp 解析组件。
	ErrYTDLPNotFound = errors.New("runtime: 未找到解析组件 yt-dlp，请将官方 yt-dlp 放入应用工具目录或系统 PATH，或设置 MEDIA_FETCHER_YTDLP_PATH")
	// ErrJSRuntimeNotFound 表示未找到兼容的 JavaScript 运行时（Deno >= 2.3 / Node >= 22 / QuickJS-NG >= 0.12）。
	ErrJSRuntimeNotFound = errors.New("runtime: 未找到兼容的 JavaScript 运行时（支持 Deno >= 2.3、Node >= 22 或 QuickJS-NG >= 0.12），请配置至应用工具目录或系统 PATH")

	semverRE = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)
)

// ManagedResolver 为未来 managed 更新版本预留的只读解析钩子（当前阶段不实现自动更新）。
type ManagedResolver interface {
	ResolveManagedYTDLP() (path string, ok bool)
}

// Options 控制外部工具发现行为，供 CLI、桌面端与测试复用。
type Options struct {
	ExeDir        string          // 覆写主程序所在目录（为空时自动读取 os.Executable）
	AppDataDir    string          // 覆写应用数据根目录（为空时使用 %APPDATA%/media-fetcher 或 ~/.media-fetcher）
	ExtraToolDirs []string        // 额外受信任工具目录（测试或自定义部署）
	Managed       ManagedResolver // 预留：未来 managed 更新解析器

	FFmpegPath    string // 显式指定 ffmpeg 路径
	FFprobePath   string // 显式指定 ffprobe 路径
	YTDLPPath     string // 显式指定 yt-dlp 路径
	JSRuntime     string // 显式指定 JS 运行时类型（deno / node / quickjs）
	JSRuntimePath string // 显式指定 JS 运行时路径
	CacheDir      string // yt-dlp 缓存目录
}

// ResolvedTool 描述单个已解析外部工具的位置与来源。
type ResolvedTool struct {
	Kind    string // 如 "ffmpeg" / "yt-dlp" / "deno" / "node" / "quickjs"
	Path    string
	Version string
	Source  Source
}

// MediaTools 保存配对发现的 FFmpeg 与 ffprobe，确保两者来自同一套组件目录。
type MediaTools struct {
	FFmpeg  ResolvedTool
	FFprobe ResolvedTool
	Source  Source
}

// YouTubeTools 保存 YouTube 解析所需的 yt-dlp、JS 运行时与缓存目录。
type YouTubeTools struct {
	YTDLP    ResolvedTool
	JS       ResolvedTool
	CacheDir string
}

func ExeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func isRegularFile(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func defaultExeDir() string {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil && real != "" {
		exe = real
	}
	return filepath.Dir(exe)
}

// AppDataRoot 返回应用专用数据目录（Windows: %APPDATA%\media-fetcher）。
func AppDataRoot(override string) string {
	if strings.TrimSpace(override) != "" {
		return filepath.Clean(override)
	}
	if runtime.GOOS == "windows" {
		if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
			return filepath.Join(appData, "media-fetcher")
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".media-fetcher")
	}
	return ""
}

type scopedDir struct {
	dir    string
	source Source
}

func collectToolDirs(opts Options) []scopedDir {
	var out []scopedDir
	seen := map[string]bool{}
	add := func(d string, src Source) {
		d = strings.TrimSpace(d)
		if d == "" {
			return
		}
		clean := filepath.Clean(d)
		key := strings.ToLower(clean)
		if !seen[key] {
			seen[key] = true
			out = append(out, scopedDir{dir: clean, source: src})
		}
	}

	exeDir := strings.TrimSpace(opts.ExeDir)
	if exeDir == "" {
		exeDir = defaultExeDir()
	}
	if exeDir != "" {
		add(filepath.Join(exeDir, "tools"), SourceBundled)
		add(filepath.Join(exeDir, "bin"), SourceBundled)
		add(exeDir, SourceBundled)
	}

	if root := AppDataRoot(opts.AppDataDir); root != "" {
		add(filepath.Join(root, "tools"), SourceAppData)
	}

	for _, extra := range opts.ExtraToolDirs {
		add(extra, SourceExplicit)
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// ResolveMediaTools 按「显式覆盖 → 随包目录 → 应用专用目录 → 系统 PATH → 常见默认目录」配对发现 FFmpeg 与 ffprobe。
// 显式覆盖路径无效时直接报错，不静默回退；默认强制从同一目录配对，避免混用随包与系统版本。
func ResolveMediaTools(opts Options) (MediaTools, error) {
	expFFmpeg := firstNonEmpty(opts.FFmpegPath, os.Getenv("MEDIA_FETCHER_FFMPEG_PATH"))
	expFFprobe := firstNonEmpty(opts.FFprobePath, os.Getenv("MEDIA_FETCHER_FFPROBE_PATH"))

	if expFFmpeg != "" || expFFprobe != "" {
		return resolveExplicitMediaTools(expFFmpeg, expFFprobe)
	}

	ffName := ExeName("ffmpeg")
	fpName := ExeName("ffprobe")

	// 1. 随包目录、应用专用目录及额外受信任目录（按同目录配对）
	for _, sd := range collectToolDirs(opts) {
		subDirs := []string{filepath.Join(sd.dir, "ffmpeg"), sd.dir}
		for _, dir := range subDirs {
			ff := filepath.Join(dir, ffName)
			fp := filepath.Join(dir, fpName)
			hasFF := isRegularFile(ff)
			hasFP := isRegularFile(fp)
			if hasFF && hasFP {
				return MediaTools{
					FFmpeg:  ResolvedTool{Kind: "ffmpeg", Path: ff, Source: sd.source},
					FFprobe: ResolvedTool{Kind: "ffprobe", Path: fp, Source: sd.source},
					Source:  sd.source,
				}, nil
			}
			if hasFF != hasFP {
				return MediaTools{}, fmt.Errorf("%w: %s 目录中的 FFmpeg 与 ffprobe 组件不完整（缺失配对文件），请重新安装或修复组件", ErrFFmpegNotFound, dir)
			}
		}
	}

	// 2. 系统 PATH（强制同目录配对 FFmpeg 与 ffprobe，拒绝跨目录混用不同版本）
	var partialPathFFmpeg string
	seenPathDirs := map[string]bool{}
	for _, rawDir := range filepath.SplitList(os.Getenv("PATH")) {
		dir := strings.TrimSpace(rawDir)
		if dir == "" {
			continue
		}
		cleanDir := filepath.Clean(dir)
		key := strings.ToLower(cleanDir)
		if seenPathDirs[key] {
			continue
		}
		seenPathDirs[key] = true
		ff := filepath.Join(cleanDir, ffName)
		fp := filepath.Join(cleanDir, fpName)
		hasFF := isRegularFile(ff)
		hasFP := isRegularFile(fp)
		if hasFF && hasFP {
			return MediaTools{
				FFmpeg:  ResolvedTool{Kind: "ffmpeg", Path: ff, Source: SourceSystem},
				FFprobe: ResolvedTool{Kind: "ffprobe", Path: fp, Source: SourceSystem},
				Source:  SourceSystem,
			}, nil
		}
		if hasFF && !hasFP && partialPathFFmpeg == "" {
			partialPathFFmpeg = ff
		}
	}

	if partialPathFFmpeg != "" {
		return MediaTools{}, fmt.Errorf("%w: 已在 PATH 找到 ffmpeg (%s)，但同目录缺少配对的 %s（拒绝跨目录混用不同版本的 ffprobe）", ErrFFmpegNotFound, partialPathFFmpeg, fpName)
	}

	// 3. 常见 Windows 默认目录（同目录配对）
	if runtime.GOOS == "windows" {
		for _, dir := range []string{`C:\tool\ffmpeg\bin`, `C:\ffmpeg\bin`, `C:\Program Files\ffmpeg\bin`} {
			ff := filepath.Join(dir, ffName)
			fp := filepath.Join(dir, fpName)
			if isRegularFile(ff) && isRegularFile(fp) {
				return MediaTools{
					FFmpeg:  ResolvedTool{Kind: "ffmpeg", Path: ff, Source: SourceSystem},
					FFprobe: ResolvedTool{Kind: "ffprobe", Path: fp, Source: SourceSystem},
					Source:  SourceSystem,
				}, nil
			}
		}
	}

	return MediaTools{}, ErrFFmpegNotFound
}

func resolveExplicitMediaTools(expFFmpeg, expFFprobe string) (MediaTools, error) {
	ffName := ExeName("ffmpeg")
	fpName := ExeName("ffprobe")

	var ffPath, fpPath string
	if expFFmpeg != "" {
		if !isRegularFile(expFFmpeg) {
			return MediaTools{}, fmt.Errorf("%w: 指定的 FFmpeg 路径不存在或不可访问: %s", ErrInvalidExplicitPath, expFFmpeg)
		}
		ffPath = filepath.Clean(expFFmpeg)
	}
	if expFFprobe != "" {
		if !isRegularFile(expFFprobe) {
			return MediaTools{}, fmt.Errorf("%w: 指定的 ffprobe 路径不存在或不可访问: %s", ErrInvalidExplicitPath, expFFprobe)
		}
		fpPath = filepath.Clean(expFFprobe)
	}

	if ffPath != "" && fpPath == "" {
		sibling := filepath.Join(filepath.Dir(ffPath), fpName)
		if !isRegularFile(sibling) {
			return MediaTools{}, fmt.Errorf("%w: 已显式指定 FFmpeg (%s)，但同目录缺少配对的 %s", ErrInvalidExplicitPath, ffPath, fpName)
		}
		fpPath = sibling
	}
	if fpPath != "" && ffPath == "" {
		sibling := filepath.Join(filepath.Dir(fpPath), ffName)
		if !isRegularFile(sibling) {
			return MediaTools{}, fmt.Errorf("%w: 已显式指定 ffprobe (%s)，但同目录缺少配对的 %s", ErrInvalidExplicitPath, fpPath, ffName)
		}
		ffPath = sibling
	}

	// 若同时显式指定了来自不同目录的 FFmpeg 与 ffprobe，核验两者主版本兼容性。
	if expFFmpeg != "" && expFFprobe != "" && !strings.EqualFold(filepath.Clean(filepath.Dir(ffPath)), filepath.Clean(filepath.Dir(fpPath))) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ffLine, ffErr := runVersionLine(ctx, ffPath, "-version")
		fpLine, fpErr := runVersionLine(ctx, fpPath, "-version")
		if ffErr == nil && fpErr == nil {
			if err := CheckMediaVersionCompatibility(parseFFmpegVersion(ffLine), parseFFmpegVersion(fpLine)); err != nil {
				return MediaTools{}, fmt.Errorf("%w: %v", ErrInvalidExplicitPath, err)
			}
		}
	}

	return MediaTools{
		FFmpeg:  ResolvedTool{Kind: "ffmpeg", Path: ffPath, Source: SourceExplicit},
		FFprobe: ResolvedTool{Kind: "ffprobe", Path: fpPath, Source: SourceExplicit},
		Source:  SourceExplicit,
	}, nil
}

// ResolveYTDLP 按「显式覆盖 → 预留 managed → 随包目录 → 应用专用目录 → 系统 PATH」定位 yt-dlp。
// 显式覆盖路径无效时直接报错，不静默切换其他位置。
func ResolveYTDLP(opts Options) (ResolvedTool, error) {
	explicit := firstNonEmpty(opts.YTDLPPath, os.Getenv("MEDIA_FETCHER_YTDLP_PATH"))
	if explicit != "" {
		if !isRegularFile(explicit) {
			return ResolvedTool{}, fmt.Errorf("%w: 指定的 yt-dlp 路径不存在或不可访问: %s", ErrInvalidExplicitPath, explicit)
		}
		return ResolvedTool{Kind: "yt-dlp", Path: filepath.Clean(explicit), Source: SourceExplicit}, nil
	}

	// 预留 managed 更新钩子（03.7C.2 不实现自动更新）
	if opts.Managed != nil {
		if p, ok := opts.Managed.ResolveManagedYTDLP(); ok && isRegularFile(p) {
			return ResolvedTool{Kind: "yt-dlp", Path: filepath.Clean(p), Source: SourceManaged}, nil
		}
	}

	// 优先官方目录版 yt-dlp/yt-dlp.exe，其次单文件版 yt-dlp.exe
	names := []string{ExeName("yt-dlp")}
	if runtime.GOOS == "windows" {
		names = []string{filepath.Join("yt-dlp", "yt-dlp.exe"), "yt-dlp.exe"}
	} else {
		names = []string{filepath.Join("yt-dlp", "yt-dlp"), "yt-dlp"}
	}

	for _, sd := range collectToolDirs(opts) {
		for _, name := range names {
			candidate := filepath.Join(sd.dir, name)
			if isRegularFile(candidate) {
				return ResolvedTool{Kind: "yt-dlp", Path: candidate, Source: sd.source}, nil
			}
		}
	}

	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return ResolvedTool{Kind: "yt-dlp", Path: p, Source: SourceSystem}, nil
	}
	return ResolvedTool{}, ErrYTDLPNotFound
}

// ResolveJSRuntime 按「显式覆盖 → 随包目录（优先随附 Deno，兼容 Node/QuickJS-NG）→ 应用专用目录 → 系统 PATH」发现并校验版本。
// 显式覆盖路径无效或版本不兼容时直接报错，不静默回退。
func ResolveJSRuntime(ctx context.Context, opts Options) (ResolvedTool, error) {
	explicitPath := firstNonEmpty(opts.JSRuntimePath, os.Getenv("MEDIA_FETCHER_JS_RUNTIME_PATH"))
	explicitKind := strings.ToLower(firstNonEmpty(opts.JSRuntime, os.Getenv("MEDIA_FETCHER_JS_RUNTIME")))

	if explicitPath != "" {
		if !isRegularFile(explicitPath) {
			return ResolvedTool{}, fmt.Errorf("%w: 指定的 JS 运行时路径不存在或不可访问: %s", ErrInvalidExplicitPath, explicitPath)
		}
		kind := explicitKind
		if kind == "" {
			kind = InferJSRuntimeKind(explicitPath)
		}
		if kind == "" {
			return ResolvedTool{}, fmt.Errorf("%w: 无法识别显式指定的 JS 运行时类型（请设置 MEDIA_FETCHER_JS_RUNTIME=deno|node|quickjs）: %s", ErrInvalidExplicitPath, explicitPath)
		}
		ver, ok := VerifyJSRuntimeVersion(ctx, kind, explicitPath)
		if !ok {
			return ResolvedTool{}, fmt.Errorf("%w: 显式指定的 JS 运行时 (%s: %s) 无法执行或版本不满足要求", ErrInvalidExplicitPath, kind, explicitPath)
		}
		return ResolvedTool{Kind: kind, Path: filepath.Clean(explicitPath), Version: ver, Source: SourceExplicit}, nil
	}

	type candidate struct {
		kind   string
		path   string
		source Source
	}
	var candidates []candidate
	seen := map[string]bool{}
	add := func(kind, p string, src Source) {
		if p == "" {
			return
		}
		if explicitKind != "" && kind != explicitKind {
			return
		}
		clean := filepath.Clean(p)
		key := kind + ":" + strings.ToLower(clean)
		if !seen[key] {
			seen[key] = true
			candidates = append(candidates, candidate{kind: kind, path: clean, source: src})
		}
	}

	// 随包及应用目录：优先随附 Deno，同时兼容 Node / QuickJS-NG
	for _, sd := range collectToolDirs(opts) {
		for _, item := range []struct{ kind, bin string }{
			{"deno", ExeName("deno")},
			{"node", ExeName("node")},
			{"quickjs", ExeName("qjs")},
		} {
			p := filepath.Join(sd.dir, item.bin)
			if isRegularFile(p) {
				add(item.kind, p, sd.source)
			}
		}
	}

	// 系统 PATH：优先 JIT 运行时（Deno / Node），其次 QuickJS-NG
	for _, item := range []struct{ kind, bin string }{
		{"deno", "deno"},
		{"node", "node"},
		{"quickjs", "qjs"},
	} {
		if p, err := exec.LookPath(item.bin); err == nil {
			add(item.kind, p, SourceSystem)
		}
	}

	for _, c := range candidates {
		if ver, ok := VerifyJSRuntimeVersion(ctx, c.kind, c.path); ok {
			return ResolvedTool{Kind: c.kind, Path: c.path, Version: ver, Source: c.source}, nil
		}
	}
	return ResolvedTool{}, ErrJSRuntimeNotFound
}

// ResolveYouTubeTools 解析 YouTube 所需的 yt-dlp、JS 运行时与缓存目录。
func ResolveYouTubeTools(ctx context.Context, opts Options) (YouTubeTools, error) {
	yt, err := ResolveYTDLP(opts)
	if err != nil {
		return YouTubeTools{}, err
	}
	js, err := ResolveJSRuntime(ctx, opts)
	if err != nil {
		return YouTubeTools{}, err
	}
	cacheDir := strings.TrimSpace(opts.CacheDir)
	if cacheDir == "" {
		if root := AppDataRoot(opts.AppDataDir); root != "" {
			cacheDir = filepath.Join(root, "cache", "yt-dlp")
		}
	}
	return YouTubeTools{
		YTDLP:    yt,
		JS:       js,
		CacheDir: cacheDir,
	}, nil
}

// InferJSRuntimeKind 根据可执行文件名推断 JS 运行时种类。
func InferJSRuntimeKind(path string) string {
	base := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasPrefix(base, "deno"):
		return "deno"
	case strings.HasPrefix(base, "node"):
		return "node"
	case strings.HasPrefix(base, "qjs"):
		return "quickjs"
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

// VerifyJSRuntimeVersion 执行 `--version` 并核验是否满足 yt-dlp EJS 最低版本要求（带文件元数据缓存）。
func VerifyJSRuntimeVersion(ctx context.Context, kind, binPath string) (string, bool) {
	clean := filepath.Clean(binPath)
	cacheKey := kind + ":" + strings.ToLower(clean)
	if info, err := os.Stat(clean); err == nil && !info.IsDir() {
		jsVersionCacheMu.Lock()
		if cached, hit := jsVersionCache[cacheKey]; hit && cached.modTimeUnix == info.ModTime().UnixNano() && cached.size == info.Size() {
			jsVersionCacheMu.Unlock()
			return cached.version, cached.ok
		}
		jsVersionCacheMu.Unlock()

		vCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(vCtx, clean, "--version")
		HideWindow(cmd)
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return "", false
		}
		ver, ok := ParseAndCheckJSVersion(kind, string(out))
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
	return "", false
}

// ParseAndCheckJSVersion 解析并核验 JS 运行时版本输出。
func ParseAndCheckJSVersion(kind, rawOutput string) (string, bool) {
	text := strings.TrimSpace(rawOutput)
	if text == "" {
		return "", false
	}
	switch kind {
	case "quickjs":
		if m := semverRE.FindStringSubmatch(text); len(m) >= 3 {
			major, _ := strconv.Atoi(m[1])
			minor, _ := strconv.Atoi(m[2])
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
		if m := semverRE.FindStringSubmatch(text); len(m) >= 3 {
			major, _ := strconv.Atoi(m[1])
			if major >= 22 {
				return m[0], true
			}
		}
		return "", false
	case "deno":
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
