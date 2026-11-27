package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type MediaProbe struct {
	Format struct {
		Name     string `json:"format_name"`
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		Type  string `json:"codec_type"`
		Codec string `json:"codec_name"`
	} `json:"streams"`
}

func ProbeMedia(ctx context.Context, path string) (MediaProbe, error) {
	var p MediaProbe
	tool, err := exec.LookPath("ffprobe")
	if err != nil {
		ffmpeg, findErr := FindFFmpeg()
		if findErr != nil {
			return p, err
		}
		tool = filepath.Join(filepath.Dir(ffmpeg), "ffprobe"+filepath.Ext(ffmpeg))
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, tool, "-v", "error", "-show_entries", "format=format_name,duration:stream=codec_type,codec_name", "-of", "json", "-i", path).Output()
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(data, &p)
	return p, err
}

func (p MediaProbe) Seconds() float64 {
	n, err := strconv.ParseFloat(p.Format.Duration, 64)
	if err != nil || n <= 0 || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0
	}
	return n
}

// NormalizeVideoCodec 将平台或 ffprobe 编码标识归类为标准视频编码族。
func NormalizeVideoCodec(codec string) string {
	c := strings.ToLower(strings.TrimSpace(codec))
	switch {
	case strings.HasPrefix(c, "avc"), strings.HasPrefix(c, "h264"), c == "x264":
		return "avc"
	case strings.HasPrefix(c, "hev"), strings.HasPrefix(c, "hvc"), strings.HasPrefix(c, "h265"), c == "hevc":
		return "hevc"
	case strings.HasPrefix(c, "av01"), c == "av1":
		return "av1"
	case strings.HasPrefix(c, "vp09"), strings.HasPrefix(c, "vp9"):
		return "vp9"
	case strings.HasPrefix(c, "vp08"), strings.HasPrefix(c, "vp8"):
		return "vp8"
	case c == "mpeg4", strings.HasPrefix(c, "mp4v"):
		return "mpeg4"
	default:
		return ""
	}
}

// NormalizeAudioCodec 将平台或 ffprobe 编码标识归类为标准音频编码族。
func NormalizeAudioCodec(codec string) string {
	c := strings.ToLower(strings.TrimSpace(codec))
	switch {
	case c == "mp4a.40.34", c == "mp3":
		return "mp3"
	case strings.HasPrefix(c, "mp4a"), c == "aac":
		return "aac"
	case strings.HasPrefix(c, "opus"):
		return "opus"
	case strings.HasPrefix(c, "vorbis"):
		return "vorbis"
	case strings.HasPrefix(c, "flac"):
		return "flac"
	case c == "ec-3", c == "eac3", strings.HasPrefix(c, "ec-3"):
		return "eac3"
	case c == "ac-3", c == "ac3", strings.HasPrefix(c, "ac-3"):
		return "ac3"
	default:
		return ""
	}
}

// ResolveVideoContainer 依据音视频编码组合决定无损输出容器（mp4 / webm / mkv），
// 优先保持原编码不做静默转码，对未知编码组合明确报错而非猜测。
func ResolveVideoContainer(videoCodec, audioCodec string) (string, error) {
	v := NormalizeVideoCodec(videoCodec)
	if v == "" {
		return "", fmt.Errorf("不支持的视频编码，无法确定输出容器: %q", videoCodec)
	}

	// 仅视频模式
	if strings.TrimSpace(audioCodec) == "" {
		switch v {
		case "avc", "hevc", "av1", "mpeg4":
			return "mp4", nil
		case "vp9", "vp8":
			return "webm", nil
		default:
			return "", fmt.Errorf("不支持的视频编码容器决策: %q", videoCodec)
		}
	}

	a := NormalizeAudioCodec(audioCodec)
	if a == "" {
		return "", fmt.Errorf("不支持的音频编码，无法确定输出容器: %q", audioCodec)
	}

	// 完整视频模式：优先原生无损容器，跨容器组合无损封装至 MKV
	switch v {
	case "avc", "hevc", "av1", "mpeg4":
		switch a {
		case "aac", "ac3", "eac3", "flac", "mp3":
			return "mp4", nil
		case "opus", "vorbis":
			return "mkv", nil
		}
	case "vp9", "vp8":
		switch a {
		case "opus", "vorbis":
			return "webm", nil
		case "aac", "ac3", "eac3", "flac", "mp3":
			return "mkv", nil
		}
	}
	return "", fmt.Errorf("不支持的音视频编码组合: video=%q, audio=%q", videoCodec, audioCodec)
}

// ResolveVideoContainerFromFiles 在元数据编码缺失时回退用 ffprobe 探测实际源轨道再决策输出容器。
func ResolveVideoContainerFromFiles(ctx context.Context, videoCodec, audioCodec, videoPath, audioPath string) (string, error) {
	vCodec := strings.TrimSpace(videoCodec)
	aCodec := strings.TrimSpace(audioCodec)
	if vCodec == "" && videoPath != "" {
		if vp, err := ProbeMedia(ctx, videoPath); err == nil {
			vCodec = firstStreamCodec(vp, "video")
		}
	}
	if audioPath != "" && aCodec == "" {
		if ap, err := ProbeMedia(ctx, audioPath); err == nil {
			aCodec = firstStreamCodec(ap, "audio")
		}
	}
	return ResolveVideoContainer(vCodec, aCodec)
}

func matchesContainerFormat(formatName, target string) bool {
	tokens := "," + strings.ToLower(formatName) + ","
	switch target {
	case "mp4", "m4a":
		return strings.Contains(tokens, ",mp4,")
	case "webm":
		return strings.Contains(tokens, ",webm,")
	case "mkv":
		return strings.Contains(tokens, ",matroska,")
	case "opus":
		return strings.Contains(tokens, ",ogg,")
	default:
		return strings.ToLower(formatName) == target || strings.Contains(tokens, ","+target+",")
	}
}

// PrepareSingleTrack 返回已确认的原轨道，或工作目录中的封装/转码结果。
func PrepareSingleTrack(ctx context.Context, path, output, format string, stage func(string)) (string, error) {
	switch format {
	case "mp4", "webm", "mkv", "m4a", "mp3", "flac", "eac3", "ac3", "opus":
	default:
		return "", fmt.Errorf("不支持的输出格式")
	}
	p, probeErr := ProbeMedia(ctx, path)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if probeErr != nil {
		return "", fmt.Errorf("媒体探测失败: %w", probeErr)
	}
	if len(p.Streams) != 1 || p.Seconds() <= 0 {
		return "", fmt.Errorf("媒体轨道或时长无效")
	}
	if (format == "flac" || format == "eac3" || format == "ac3" || format == "opus") && p.Streams[0].Codec == format && (p.Format.Name == format || format == "opus" && p.Format.Name == "ogg") {
		return path, nil
	}
	if format == "mp3" && p.Streams[0].Codec == "mp3" && p.Format.Name == "mp3" {
		return path, nil
	}
	if format == "webm" && p.Streams[0].Type == "video" && matchesContainerFormat(p.Format.Name, "webm") && (p.Streams[0].Codec == "vp9" || p.Streams[0].Codec == "vp8") {
		return path, nil
	}
	if format != "mp3" && p.Seconds() > 0 && len(p.Streams) == 1 && matchesContainerFormat(p.Format.Name, "mp4") {
		stream := p.Streams[0]
		if format == "mp4" && stream.Type == "video" || format == "m4a" && stream.Type == "audio" && stream.Codec == "aac" {
			return path, nil
		}
	}
	ffmpeg, err := FindFFmpeg()
	if err != nil {
		return "", fmt.Errorf("%s 输出需要 FFmpeg: %w", format, err)
	}
	args := []string{"-nostdin", "-v", "error", "-n", "-i", path}
	switch format {
	case "mp4":
		stage("remuxing")
		args = append(args, "-map", "0:v:0", "-an", "-c:v", "copy", "-movflags", "+faststart")
	case "webm", "mkv":
		stage("remuxing")
		args = append(args, "-map", "0:v:0", "-an", "-c:v", "copy")
	case "m4a":
		if len(p.Streams) == 1 && p.Streams[0].Type == "audio" && p.Streams[0].Codec != "aac" {
			stage("transcoding")
			args = append(args, "-map", "0:a:0", "-c:a", "aac", "-b:a", "192k")
		} else {
			stage("remuxing")
			args = append(args, "-map", "0:a:0", "-c:a", "copy")
		}
	case "flac", "eac3", "ac3", "opus":
		stage("remuxing")
		args = append(args, "-map", "0:a:0", "-vn", "-c:a", "copy")
	case "mp3":
		stage("transcoding")
		args = append(args, "-map", "0:a:0", "-c:a", "libmp3lame", "-q:a", "2")
	}
	data, err := exec.CommandContext(ctx, ffmpeg, append(args, output)...).CombinedOutput()
	if err != nil {
		_ = os.Remove(output)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("%s 处理失败，请检查 FFmpeg 是否支持所需编码: %.400s", format, data)
	}
	if info, err := os.Stat(output); err != nil || info.Size() == 0 {
		return "", fmt.Errorf("%s 输出为空", format)
	}
	verified, verifyErr := ProbeMedia(ctx, output)
	if verifyErr != nil || len(verified.Streams) != 1 || verified.Seconds() <= 0 || !matchesOutput(verified, format, p.Streams[0].Codec) {
		os.Remove(output)
		return "", fmt.Errorf("输出验证失败")
	}
	return output, nil
}

// OriginalAudioFormat 根据真实探测结果选择容器，不根据下载链接或文件后缀猜测。
func OriginalAudioFormat(ctx context.Context, path string) (string, error) {
	p, err := ProbeMedia(ctx, path)
	if err != nil {
		return "", fmt.Errorf("原始音频探测失败: %w", err)
	}
	if len(p.Streams) != 1 || p.Streams[0].Type != "audio" {
		return "", fmt.Errorf("原始输出需要独立音频轨道")
	}
	switch p.Streams[0].Codec {
	case "aac":
		return "m4a", nil
	case "flac", "eac3", "ac3", "opus", "mp3":
		return p.Streams[0].Codec, nil
	}
	return "", fmt.Errorf("不支持的原始音频编码: %s", p.Streams[0].Codec)
}

// matchesOutput 校验输出策略要求的实际类型、编码和容器。
func matchesOutput(p MediaProbe, format, sourceCodec string) bool {
	if len(p.Streams) != 1 {
		return false
	}
	s := p.Streams[0]
	switch format {
	case "mp4", "webm", "mkv":
		return s.Type == "video" && s.Codec == sourceCodec && matchesContainerFormat(p.Format.Name, format)
	}
	if s.Type != "audio" {
		return false
	}
	switch format {
	case "m4a":
		return s.Codec == "aac" && matchesContainerFormat(p.Format.Name, "mp4")
	case "opus":
		return s.Codec == "opus" && matchesContainerFormat(p.Format.Name, "opus")
	default:
		return s.Codec == format && p.Format.Name == format
	}
}
