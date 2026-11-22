package downloader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

var (
	ErrFFmpegNotFound = errors.New("muxer: ffmpeg executable not found")
	ErrMuxFailed      = errors.New("muxer: ffmpeg execution failed")
)

// FindFFmpeg attempts to locate ffmpeg in common paths or PATH.
func FindFFmpeg() (string, error) {
	// 1. Check PATH
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, nil
	}

	// 2. Common Windows paths
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

// MuxVideoAudio uses FFmpeg Stream Copy (-c copy) to merge video and audio streams
// into a final MP4 container without re-encoding.
// It strictly uses exec.CommandContext with parameter arrays, avoiding any shell injection.
func MuxVideoAudio(ctx context.Context, ffmpegPath, videoPath, audioPath, outputPath string) error {
	if ffmpegPath == "" {
		p, err := FindFFmpeg()
		if err != nil {
			return err
		}
		ffmpegPath = p
	}

	if _, err := os.Stat(videoPath); err != nil {
		return fmt.Errorf("muxer: video file not found: %w", err)
	}
	if _, err := os.Stat(audioPath); err != nil {
		return fmt.Errorf("muxer: audio file not found: %w", err)
	}

	// Explicit argument array - no shell interpolation!
	args := []string{
		"-y",            // Overwrite output
		"-i", videoPath, // Input 0: video
		"-i", audioPath, // Input 1: audio
		"-c", "copy", // Stream copy without re-encoding
		"-movflags", "+faststart", // Optimize for streaming playback
		outputPath,
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v (stderr: %s)", ErrMuxFailed, err, stderr.String())
	}

	// Validate output file
	stat, err := os.Stat(outputPath)
	if err != nil || stat.Size() == 0 {
		return fmt.Errorf("%w: output file missing or empty", ErrMuxFailed)
	}

	return nil
}
