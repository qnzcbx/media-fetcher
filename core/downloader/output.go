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
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
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

// PrepareSingleTrack 返回已确认的原轨道，或工作目录中的封装结果。
func PrepareSingleTrack(ctx context.Context, path, output, format string, stage func(string)) (string, error) {
	switch format {
	case "mp4", "m4a", "mp3":
	default:
		return "", fmt.Errorf("不支持的输出格式")
	}
	p, err := ProbeMedia(ctx, path)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err == nil && format != "mp3" && p.Seconds() > 0 && len(p.Streams) == 1 && strings.Contains(","+p.Format.Name+",", ",mp4,") {
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
		args = append(args, "-map", "0:v:0", "-an", "-c:v", "copy")
	case "m4a":
		if err == nil && len(p.Streams) == 1 && p.Streams[0].Type == "audio" && p.Streams[0].Codec != "aac" {
			stage("transcoding")
			args = append(args, "-map", "0:a:0", "-c:a", "aac", "-b:a", "192k")
		} else {
			stage("remuxing")
			args = append(args, "-map", "0:a:0", "-c:a", "copy")
		}
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
	return output, nil
}
