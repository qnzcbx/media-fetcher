package runtime

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ComponentStatus 描述单项运行组件的对外状态（用于设置页状态展示与按需诊断）。
type ComponentStatus struct {
	Key       string `json:"key"`       // "ytdlp" | "jsRuntime" | "ffmpeg"
	Name      string `json:"name"`      // 面向用户的组件名
	Available bool   `json:"available"` // 是否可执行且通过基础版本检查
	Version   string `json:"version"`   // 人类可读版本号
	Source    Source `json:"source"`    // explicit | managed | bundled | appdata | system | missing
	Kind      string `json:"kind"`      // 如 deno / node / quickjs / yt-dlp / ffmpeg
	Detail    string `json:"detail"`    // 按需展示的简明诊断信息或错误说明
	Path      string `json:"path"`      // 仅用于按需展开的诊断详情，不直接平铺于主设置界面
	PairPath  string `json:"pairPath"`  // 仅 FFmpeg 项：配对的 ffprobe 路径
}

// StatusSnapshot 汇总全部三类运行组件的实时状态。
type StatusSnapshot struct {
	YTDLP     ComponentStatus `json:"ytdlp"`
	JSRuntime ComponentStatus `json:"jsRuntime"`
	FFmpeg    ComponentStatus `json:"ffmpeg"`
}

// InspectStatus 检测下载引擎（yt-dlp）、JS 运行时与媒体处理（FFmpeg / ffprobe）的可用状态与版本。
func InspectStatus(ctx context.Context, opts Options) StatusSnapshot {
	return StatusSnapshot{
		YTDLP:     inspectYTDLP(ctx, opts),
		JSRuntime: inspectJS(ctx, opts),
		FFmpeg:    inspectMedia(ctx, opts),
	}
}

func runVersionLine(ctx context.Context, bin string, arg string) (string, error) {
	vCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(vCtx, bin, arg)
	HideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			return s, nil
		}
	}
	return "", fmt.Errorf("empty version output")
}

func parseFFmpegVersion(line string) string {
	// e.g. "ffmpeg version n9.0.2-39-g7adfbe40dc-20261031 Copyright ..."
	fields := strings.Fields(line)
	for i := 0; i < len(fields)-1; i++ {
		if strings.EqualFold(fields[i], "version") {
			return fields[i+1]
		}
	}
	return strings.TrimSpace(line)
}

func inspectYTDLP(ctx context.Context, opts Options) ComponentStatus {
	st := ComponentStatus{
		Key:    "ytdlp",
		Name:   "下载引擎（yt-dlp）",
		Kind:   "yt-dlp",
		Source: SourceMissing,
	}
	tool, err := ResolveYTDLP(opts)
	if err != nil {
		st.Detail = err.Error()
		return st
	}
	st.Source = tool.Source
	st.Path = tool.Path
	ver, runErr := runVersionLine(ctx, tool.Path, "--version")
	if runErr != nil {
		st.Detail = fmt.Sprintf("yt-dlp 无法启动或文件已损坏: %v", runErr)
		return st
	}
	st.Available = true
	st.Version = ver
	st.Detail = fmt.Sprintf("yt-dlp %s", ver)
	return st
}

func inspectJS(ctx context.Context, opts Options) ComponentStatus {
	st := ComponentStatus{
		Key:    "jsRuntime",
		Name:   "JavaScript 运行时",
		Source: SourceMissing,
	}
	tool, err := ResolveJSRuntime(ctx, opts)
	if err != nil {
		st.Detail = err.Error()
		return st
	}
	st.Available = true
	st.Kind = tool.Kind
	st.Version = tool.Version
	st.Source = tool.Source
	st.Path = tool.Path
	st.Detail = fmt.Sprintf("%s %s", displayJSKind(tool.Kind), tool.Version)
	return st
}

func displayJSKind(kind string) string {
	switch kind {
	case "deno":
		return "Deno"
	case "node":
		return "Node.js"
	case "quickjs":
		return "QuickJS-NG"
	default:
		return kind
	}
}

func inspectMedia(ctx context.Context, opts Options) ComponentStatus {
	st := ComponentStatus{
		Key:    "ffmpeg",
		Name:   "媒体处理（FFmpeg / ffprobe）",
		Kind:   "ffmpeg",
		Source: SourceMissing,
	}
	tools, err := ResolveMediaTools(opts)
	if err != nil {
		st.Detail = err.Error()
		return st
	}
	st.Source = tools.Source
	st.Path = tools.FFmpeg.Path
	st.PairPath = tools.FFprobe.Path

	ffLine, ffErr := runVersionLine(ctx, tools.FFmpeg.Path, "-version")
	if ffErr != nil {
		st.Detail = fmt.Sprintf("FFmpeg 无法启动或文件已损坏: %v", ffErr)
		return st
	}
	fpLine, fpErr := runVersionLine(ctx, tools.FFprobe.Path, "-version")
	if fpErr != nil {
		st.Detail = fmt.Sprintf("ffprobe 无法启动或文件已损坏: %v", fpErr)
		return st
	}
	ffVer := parseFFmpegVersion(ffLine)
	fpVer := parseFFmpegVersion(fpLine)
	st.Available = true
	st.Version = ffVer
	if fpVer != "" && fpVer != ffVer {
		st.Detail = fmt.Sprintf("FFmpeg %s / ffprobe %s", ffVer, fpVer)
	} else {
		st.Detail = fmt.Sprintf("FFmpeg & ffprobe %s", ffVer)
	}
	return st
}
