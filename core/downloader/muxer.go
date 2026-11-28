package downloader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	coreruntime "media-fetcher/core/runtime"
	"media-fetcher/pkg/fsutil"
)

var (
	// ErrFFmpegNotFound 表示在系统中未找到可用的 ffmpeg 可执行文件。
	ErrFFmpegNotFound = errors.New("muxer: 未找到 ffmpeg 可执行文件")
	// ErrMuxFailed 表示 ffmpeg 执行混流失败。
	ErrMuxFailed = errors.New("muxer: ffmpeg 封装执行失败")
)

// FindFFmpeg 按统一优先级（显式覆盖 → 随包目录 → 应用工具目录 → 系统 PATH）定位与 ffprobe 配对的 ffmpeg。
func FindFFmpeg() (string, error) {
	tools, err := coreruntime.ResolveMediaTools(coreruntime.Options{})
	if err != nil {
		if errors.Is(err, coreruntime.ErrInvalidExplicitPath) {
			return "", err
		}
		return "", fmt.Errorf("%w: %v", ErrFFmpegNotFound, err)
	}
	return tools.FFmpeg.Path, nil
}

// FindFFprobe 按统一优先级定位与 ffmpeg 配对的 ffprobe。
func FindFFprobe() (string, error) {
	tools, err := coreruntime.ResolveMediaTools(coreruntime.Options{})
	if err != nil {
		return "", err
	}
	return tools.FFprobe.Path, nil
}

// MuxVideoAudio 使用 FFmpeg Stream Copy (-c copy) 将音视频轨道无损封装为目标容器（MP4 / WebM / MKV），
// 不进行静默二次重编码。严格采用 os/exec 参数数组传参杜绝注入，并通过 ffprobe 校验输出容器、轨道数、编码与时长，
// 再通过独占临时文件原子发布，失败或取消时自动清理临时文件，绝不产生残缺损坏文件，也不破坏既有文件。
func MuxVideoAudio(ctx context.Context, ffmpegPath, videoPath, audioPath, outputPath string) error {
	if ffmpegPath == "" {
		p, err := FindFFmpeg()
		if err != nil {
			return err
		}
		ffmpegPath = p
	}

	if _, err := os.Stat(videoPath); err != nil {
		return fmt.Errorf("muxer: 视频源文件不存在: %w", err)
	}
	if _, err := os.Stat(audioPath); err != nil {
		return fmt.Errorf("muxer: 音频源文件不存在: %w", err)
	}

	outDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("muxer: 创建输出目录失败: %w", err)
	}

	// 临时文件输出路径，杜绝直接向最终目标路径覆写
	ext := strings.ToLower(filepath.Ext(outputPath))
	if ext == "" {
		ext = ".mp4"
	}
	format := strings.TrimPrefix(ext, ".")
	switch format {
	case "mp4", "webm", "mkv":
	default:
		return fmt.Errorf("%w: 不支持的视频封装容器 %q", ErrMuxFailed, format)
	}

	tmpMuxPath := filepath.Join(outDir, fmt.Sprintf(".tmp_mux.%d.%d%s", os.Getpid(), time.Now().UnixNano(), ext))

	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpMuxPath)
		}
	}()

	// 显式参数数组传参 — 杜绝 Shell 字符串拼接风险
	args := []string{
		"-nostdin",
		"-y",            // 允许覆盖临时目标
		"-i", videoPath, // 视频输入
		"-i", audioPath, // 音频输入
		"-map", "0:v:0", "-map", "1:a:0",
	}
	if format == "mp4" {
		args = append(args, "-strict", "-2", "-c", "copy", "-movflags", "+faststart")
	} else {
		// WebM / MKV 不使用 MP4 专属的 -movflags +faststart 参数
		args = append(args, "-c", "copy")
	}
	args = append(args, tmpMuxPath)

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	coreruntime.HideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %v (stderr: %s)", ErrMuxFailed, err, stderr.String())
	}

	// 验证临时文件非空
	stat, err := os.Stat(tmpMuxPath)
	if err != nil || stat.Size() == 0 {
		return fmt.Errorf("%w: 混流输出文件缺失或为空", ErrMuxFailed)
	}

	// 使用 ffprobe 严格校验输出容器、轨道数、编码与时长（不仅以 FFmpeg 退出码 0 判成功）
	if err := verifyMuxedOutput(ctx, videoPath, audioPath, tmpMuxPath, format); err != nil {
		return err
	}

	// 检查 Context 取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 原子替换到最终目标路径
	if err := fsutil.PublishFile(tmpMuxPath, outputPath, fsutil.StrategyReplace); err != nil {
		return fmt.Errorf("muxer: 原子发布最终文件失败: %w", err)
	}

	success = true
	return nil
}

func verifyMuxedOutput(ctx context.Context, videoPath, audioPath, outputPath, format string) error {
	outProbe, err := ProbeMedia(ctx, outputPath)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: ffprobe 输出验证失败: %v", ErrMuxFailed, err)
	}
	if outProbe.Seconds() <= 0 || !matchesContainerFormat(outProbe.Format.Name, format) {
		return fmt.Errorf("%w: 输出容器或时长校验失败 (format=%q, duration=%q)", ErrMuxFailed, outProbe.Format.Name, outProbe.Format.Duration)
	}

	var outVideoCodec, outAudioCodec string
	var videoCount, audioCount int
	for _, s := range outProbe.Streams {
		switch s.Type {
		case "video":
			videoCount++
			outVideoCodec = s.Codec
		case "audio":
			audioCount++
			outAudioCodec = s.Codec
		}
	}
	if videoCount != 1 || audioCount != 1 {
		return fmt.Errorf("%w: 输出轨道数量异常 (video=%d, audio=%d)", ErrMuxFailed, videoCount, audioCount)
	}

	// 与源轨道核对编码一致性，确认未发生静默转码；源轨道探测失败或缺失编码时不得跳过校验
	vp, err := ProbeMedia(ctx, videoPath)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: ffprobe 源视频轨道校验失败: %v", ErrMuxFailed, err)
	}
	srcV := firstStreamCodec(vp, "video")
	if srcV == "" {
		return fmt.Errorf("%w: 源视频文件缺少有效视频轨道编码", ErrMuxFailed)
	}
	if srcV != outVideoCodec {
		return fmt.Errorf("%w: 视频编码在封装中发生非预期变化 (%s -> %s)", ErrMuxFailed, srcV, outVideoCodec)
	}

	ap, err := ProbeMedia(ctx, audioPath)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: ffprobe 源音频轨道校验失败: %v", ErrMuxFailed, err)
	}
	srcA := firstStreamCodec(ap, "audio")
	if srcA == "" {
		return fmt.Errorf("%w: 源音频文件缺少有效音频轨道编码", ErrMuxFailed)
	}
	if srcA != outAudioCodec {
		return fmt.Errorf("%w: 音频编码在封装中发生非预期变化 (%s -> %s)", ErrMuxFailed, srcA, outAudioCodec)
	}
	return nil
}

func firstStreamCodec(p MediaProbe, kind string) string {
	for _, s := range p.Streams {
		if s.Type == kind {
			return s.Codec
		}
	}
	return ""
}
