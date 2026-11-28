package runtime

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

type stubManaged struct {
	path string
	ok   bool
}

func (s stubManaged) ResolveManagedYTDLP() (string, bool) { return s.path, s.ok }

func writeDummyFile(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func clearRuntimeEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"MEDIA_FETCHER_FFMPEG_PATH",
		"MEDIA_FETCHER_FFPROBE_PATH",
		"MEDIA_FETCHER_YTDLP_PATH",
		"MEDIA_FETCHER_JS_RUNTIME",
		"MEDIA_FETCHER_JS_RUNTIME_PATH",
	} {
		t.Setenv(k, "")
	}
}

func TestResolveMediaTools_BundledFirstAndPaired(t *testing.T) {
	clearRuntimeEnv(t)
	exeDir := t.TempDir()
	appData := t.TempDir()

	bundledFF := writeDummyFile(t, filepath.Join(exeDir, "tools", "ffmpeg", ExeName("ffmpeg")))
	bundledFP := writeDummyFile(t, filepath.Join(exeDir, "tools", "ffmpeg", ExeName("ffprobe")))
	writeDummyFile(t, filepath.Join(appData, "tools", ExeName("ffmpeg")))
	writeDummyFile(t, filepath.Join(appData, "tools", ExeName("ffprobe")))

	got, err := ResolveMediaTools(Options{ExeDir: exeDir, AppDataDir: appData})
	if err != nil {
		t.Fatalf("ResolveMediaTools unexpected err: %v", err)
	}
	if got.Source != SourceBundled || got.FFmpeg.Path != bundledFF || got.FFprobe.Path != bundledFP {
		t.Fatalf("expected paired bundled tools, got: %+v", got)
	}
}

func TestResolveMediaTools_PartialBundledRejectedNoSilentMix(t *testing.T) {
	clearRuntimeEnv(t)
	exeDir := t.TempDir()
	appData := t.TempDir()

	// 随包目录仅有 ffmpeg 缺少配对 ffprobe，不得静默混用 appdata/PATH 的 ffprobe
	writeDummyFile(t, filepath.Join(exeDir, "tools", "ffmpeg", ExeName("ffmpeg")))
	writeDummyFile(t, filepath.Join(appData, "tools", ExeName("ffmpeg")))
	writeDummyFile(t, filepath.Join(appData, "tools", ExeName("ffprobe")))

	_, err := ResolveMediaTools(Options{ExeDir: exeDir, AppDataDir: appData})
	if !errors.Is(err, ErrFFmpegNotFound) {
		t.Fatalf("expected ErrFFmpegNotFound for partial bundled pair, got: %v", err)
	}
}

func TestResolveMediaTools_InvalidExplicitDoesNotFallback(t *testing.T) {
	clearRuntimeEnv(t)
	exeDir := t.TempDir()
	writeDummyFile(t, filepath.Join(exeDir, "tools", "ffmpeg", ExeName("ffmpeg")))
	writeDummyFile(t, filepath.Join(exeDir, "tools", "ffmpeg", ExeName("ffprobe")))

	_, err := ResolveMediaTools(Options{
		ExeDir:     exeDir,
		FFmpegPath: filepath.Join(exeDir, "missing-ffmpeg.exe"),
	})
	if !errors.Is(err, ErrInvalidExplicitPath) {
		t.Fatalf("expected ErrInvalidExplicitPath, got: %v", err)
	}

	// 显式指定 ffmpeg 但同目录缺少 ffprobe 时必须报错，不混用随包或系统 ffprobe
	soloDir := t.TempDir()
	soloFF := writeDummyFile(t, filepath.Join(soloDir, ExeName("ffmpeg")))
	_, err = ResolveMediaTools(Options{
		ExeDir:     exeDir,
		FFmpegPath: soloFF,
	})
	if !errors.Is(err, ErrInvalidExplicitPath) {
		t.Fatalf("expected ErrInvalidExplicitPath when sibling ffprobe missing, got: %v", err)
	}
}

func TestResolveYTDLP_PriorityAndInvalidExplicit(t *testing.T) {
	clearRuntimeEnv(t)
	exeDir := t.TempDir()
	appData := t.TempDir()
	managedDir := t.TempDir()

	bundledOnedir := writeDummyFile(t, filepath.Join(exeDir, "tools", "yt-dlp", ExeName("yt-dlp")))
	writeDummyFile(t, filepath.Join(exeDir, "tools", ExeName("yt-dlp")))
	writeDummyFile(t, filepath.Join(appData, "tools", ExeName("yt-dlp")))
	managedExe := writeDummyFile(t, filepath.Join(managedDir, ExeName("yt-dlp")))

	// 1. 随包优先目录版
	got, err := ResolveYTDLP(Options{ExeDir: exeDir, AppDataDir: appData})
	if err != nil || got.Source != SourceBundled || got.Path != bundledOnedir {
		t.Fatalf("expected bundled onedir yt-dlp, got=%+v err=%v", got, err)
	}

	// 2. 预留 managed 钩子优先于随包
	got, err = ResolveYTDLP(Options{ExeDir: exeDir, AppDataDir: appData, Managed: stubManaged{path: managedExe, ok: true}})
	if err != nil || got.Source != SourceManaged || got.Path != managedExe {
		t.Fatalf("expected managed yt-dlp, got=%+v err=%v", got, err)
	}

	// 3. 显式覆盖路径无效必须直接报错，不得回退随包
	_, err = ResolveYTDLP(Options{ExeDir: exeDir, YTDLPPath: filepath.Join(exeDir, "nonexistent.exe")})
	if !errors.Is(err, ErrInvalidExplicitPath) {
		t.Fatalf("expected ErrInvalidExplicitPath, got: %v", err)
	}
}

func TestResolveJSRuntime_InvalidExplicitDoesNotFallback(t *testing.T) {
	clearRuntimeEnv(t)
	_, err := ResolveJSRuntime(context.Background(), Options{
		JSRuntimePath: filepath.Join(t.TempDir(), "missing-deno.exe"),
	})
	if !errors.Is(err, ErrInvalidExplicitPath) {
		t.Fatalf("expected ErrInvalidExplicitPath, got: %v", err)
	}
}

func TestInspectStatus_CorruptBinaryReportsUnavailable(t *testing.T) {
	clearRuntimeEnv(t)
	exeDir := t.TempDir()
	appData := t.TempDir()

	// 写入损坏的假二进制文件（无法执行 --version / -version）
	writeDummyFile(t, filepath.Join(exeDir, "tools", "yt-dlp", ExeName("yt-dlp")))
	writeDummyFile(t, filepath.Join(exeDir, "tools", "ffmpeg", ExeName("ffmpeg")))
	writeDummyFile(t, filepath.Join(exeDir, "tools", "ffmpeg", ExeName("ffprobe")))

	st := InspectStatus(context.Background(), Options{ExeDir: exeDir, AppDataDir: appData})
	if st.YTDLP.Available {
		t.Fatalf("corrupt yt-dlp should not be reported available: %+v", st.YTDLP)
	}
	if st.YTDLP.Source != SourceBundled || st.YTDLP.Detail == "" {
		t.Fatalf("expected bundled source with error detail for corrupt yt-dlp: %+v", st.YTDLP)
	}
	if st.FFmpeg.Available {
		t.Fatalf("corrupt ffmpeg should not be reported available: %+v", st.FFmpeg)
	}
	if st.FFmpeg.Source != SourceBundled || st.FFmpeg.Detail == "" {
		t.Fatalf("expected bundled source with error detail for corrupt ffmpeg: %+v", st.FFmpeg)
	}
}

func TestParseAndCheckJSVersion(t *testing.T) {
	cases := []struct {
		kind, out string
		wantOK    bool
	}{
		{"deno", "deno 2.9.8 (stable, release, x86_64-pc-windows-msvc)\n", true},
		{"deno", "deno 2.2.9\n", false},
		{"node", "v22.22.2\n", true},
		{"node", "v20.18.0\n", false},
		{"quickjs", "0.18.0\n", true},
		{"quickjs", "0.11.0\n", false},
		{"unknown", "1.0.0\n", false},
	}
	for _, tc := range cases {
		_, ok := ParseAndCheckJSVersion(tc.kind, tc.out)
		if ok != tc.wantOK {
			t.Errorf("(%s, %q) ok=%v want=%v", tc.kind, tc.out, ok, tc.wantOK)
		}
	}
}

func TestHideWindow_WindowsSetsCreateNoWindow(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	cmd := exec.Command("cmd.exe", "/c", "echo", "ok")
	HideWindow(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow {
		t.Fatalf("HideWindow did not set SysProcAttr.HideWindow")
	}
}
