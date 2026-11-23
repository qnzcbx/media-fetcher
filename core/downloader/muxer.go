package downloader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"media-fetcher/pkg/fsutil"
)

var (
	// ErrFFmpegNotFound 表示在系统中未找到可用的 ffmpeg 可执行文件。
	ErrFFmpegNotFound = errors.New("muxer: 未找到 ffmpeg 可执行文件")
	// ErrMuxFailed 表示 ffmpeg 执行混流失败。
	ErrMuxFailed = errors.New("muxer: ffmpeg 封装执行失败")
)

// FindFFmpeg 尝试在环境变量 PATH 或常见路径中定位 ffmpeg。
func FindFFmpeg() (string, error) {
	// 1. 优先检查 PATH
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, nil
	}

	// 2. 检查常见 Windows 默认路径
	candidates := []string{
		`C:\tool\ffmpeg\bin\ffmpeg.exe`,
		`C:\ffmpeg\bin\ffmpeg.exe`,
		`C:\Program Files\ffmpeg\bin\ffmpeg.exe`,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	return "", ErrFFmpegNotFound
}

// MuxVideoAudio 使用 FFmpeg Stream Copy (-c copy) 将音视频轨道封装为最终 MP4 容器，不进行二次重编码。
// 严格采用 os/exec 参数数组传参杜绝注入，并通过独占临时文件实现原子发布，
// 失败或取消时自动清理临时文件，绝不产生残缺损坏文件，也不破坏既有文件。
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
	ext := filepath.Ext(outputPath)
	if ext == "" {
		ext = ".mp4"
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
		"-y",            // 允许覆盖临时目标
		"-i", videoPath, // 视频输入
		"-i", audioPath, // 音频输入
		"-c", "copy", // 流式复制，无需重编码
		"-movflags", "+faststart", // Web 流式快速启动优化
		tmpMuxPath,
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v (stderr: %s)", ErrMuxFailed, err, stderr.String())
	}

	// 验证临时文件生成质量
	stat, err := os.Stat(tmpMuxPath)
	if err != nil || stat.Size() == 0 {
		return fmt.Errorf("%w: 混流输出文件缺失或为空", ErrMuxFailed)
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
