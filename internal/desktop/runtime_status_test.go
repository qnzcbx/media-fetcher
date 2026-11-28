package desktop

import (
	"path/filepath"
	"testing"
)

func TestDesktopService_GetRuntimeStatus_InvalidExplicitReported(t *testing.T) {
	t.Setenv("MEDIA_FETCHER_YTDLP_PATH", filepath.Join(t.TempDir(), "missing-yt-dlp.exe"))
	t.Setenv("MEDIA_FETCHER_FFMPEG_PATH", filepath.Join(t.TempDir(), "missing-ffmpeg.exe"))
	t.Setenv("MEDIA_FETCHER_JS_RUNTIME_PATH", filepath.Join(t.TempDir(), "missing-deno.exe"))

	svc := &DesktopService{}
	st := svc.GetRuntimeStatus()
	if st.YTDLP.Available || st.YTDLP.Detail == "" {
		t.Fatalf("invalid explicit yt-dlp should be unavailable with error detail: %+v", st.YTDLP)
	}
	if st.FFmpeg.Available || st.FFmpeg.Detail == "" {
		t.Fatalf("invalid explicit ffmpeg should be unavailable with error detail: %+v", st.FFmpeg)
	}
	if st.JSRuntime.Available || st.JSRuntime.Detail == "" {
		t.Fatalf("invalid explicit js runtime should be unavailable with error detail: %+v", st.JSRuntime)
	}
}
