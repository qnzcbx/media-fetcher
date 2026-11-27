package downloader

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveVideoContainer(t *testing.T) {
	cases := []struct {
		video, audio, want string
		wantErr            bool
	}{
		{"avc1.64002A", "mp4a.40.2", "mp4", false},
		{"av01.0.13M.08", "mp4a.40.2", "mp4", false},
		{"hev1.1.6.L120.90", "fLaC", "mp4", false},
		{"avc1.640028", "ec-3", "mp4", false},
		{"vp9", "opus", "webm", false},
		{"vp09.00.51.08", "opus", "webm", false},
		{"avc1.64002A", "opus", "mkv", false},
		{"vp9", "mp4a.40.2", "mkv", false},
		{"av01.0.13M.08", "opus", "mkv", false},
		{"vp9", "", "webm", false},
		{"avc1.64002A", "", "mp4", false},
		{"av01.0.08M.08", "", "mp4", false},
		{"unknown_v", "mp4a.40.2", "", true},
		{"avc1.64002A", "unknown_a", "", true},
		{"", "mp4a.40.2", "", true},
	}
	for _, tc := range cases {
		got, err := ResolveVideoContainer(tc.video, tc.audio)
		if (err != nil) != tc.wantErr {
			t.Errorf("(%q, %q) err=%v wantErr=%v", tc.video, tc.audio, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("(%q, %q) got=%q want=%q", tc.video, tc.audio, got, tc.want)
		}
	}
}

func TestMuxVideoAudio_MP4WebMMKVLossless(t *testing.T) {
	ff, err := FindFFmpeg()
	if err != nil {
		t.Skip("FFmpeg 不可用")
	}
	d := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command(ff, append([]string{"-v", "error", "-y"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg %v: %s", err, out)
		}
	}

	vH264 := filepath.Join(d, "v_h264.mp4")
	vVP9 := filepath.Join(d, "v_vp9.webm")
	aAAC := filepath.Join(d, "a_aac.m4a")
	aOpus := filepath.Join(d, "a_opus.webm")

	run("-f", "lavfi", "-i", "color=s=64x64:r=10", "-t", "1", "-an", "-c:v", "libx264", vH264)
	run("-f", "lavfi", "-i", "color=s=64x64:r=10", "-t", "1", "-an", "-c:v", "libvpx-vp9", vVP9)
	run("-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "1", "-vn", "-c:a", "aac", aAAC)
	run("-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "1", "-vn", "-c:a", "libopus", aOpus)

	combos := []struct {
		name, video, audio, vCodec, aCodec, wantExt, wantV, wantA string
	}{
		{"avc_aac_mp4", vH264, aAAC, "avc1", "mp4a.40.2", "mp4", "h264", "aac"},
		{"vp9_opus_webm", vVP9, aOpus, "vp9", "opus", "webm", "vp9", "opus"},
		{"avc_opus_mkv", vH264, aOpus, "avc1", "opus", "mkv", "h264", "opus"},
		{"vp9_aac_mkv", vVP9, aAAC, "vp9", "mp4a.40.2", "mkv", "vp9", "aac"},
	}

	for _, tc := range combos {
		t.Run(tc.name, func(t *testing.T) {
			ext, err := ResolveVideoContainer(tc.vCodec, tc.aCodec)
			if err != nil || ext != tc.wantExt {
				t.Fatalf("容器决策错误: got=%q err=%v", ext, err)
			}
			out := filepath.Join(d, tc.name+"."+ext)
			if err := MuxVideoAudio(context.Background(), ff, tc.video, tc.audio, out); err != nil {
				t.Fatalf("混流失败: %v", err)
			}
			p, err := ProbeMedia(context.Background(), out)
			if err != nil || len(p.Streams) != 2 || !matchesContainerFormat(p.Format.Name, ext) {
				t.Fatalf("ffprobe 校验异常: %+v err=%v", p, err)
			}
			if firstStreamCodec(p, "video") != tc.wantV || firstStreamCodec(p, "audio") != tc.wantA {
				t.Fatalf("编码被改变: %+v", p.Streams)
			}
		})
	}

	// 验证仅视频 WebM 与仅音频 Opus 原始输出
	t.Run("VP9 video-only -> WebM", func(t *testing.T) {
		out, err := PrepareSingleTrack(context.Background(), vVP9, filepath.Join(d, "vonly.webm"), "webm", func(stage string) {
			if stage == "transcoding" {
				t.Fatal("仅视频 WebM 不应转码")
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		p, err := ProbeMedia(context.Background(), out)
		if err != nil || len(p.Streams) != 1 || p.Streams[0].Codec != "vp9" {
			t.Fatalf("仅视频 WebM 校验失败: %+v %v", p, err)
		}
	})

	t.Run("WebM Opus original -> .opus", func(t *testing.T) {
		origFmt, err := OriginalAudioFormat(context.Background(), aOpus)
		if err != nil || origFmt != "opus" {
			t.Fatalf("OriginalAudioFormat: %q %v", origFmt, err)
		}
		out, err := PrepareSingleTrack(context.Background(), aOpus, filepath.Join(d, "aonly.opus"), "opus", func(stage string) {
			if stage == "transcoding" {
				t.Fatal("原始 Opus 不应转码")
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		p, err := ProbeMedia(context.Background(), out)
		if err != nil || len(p.Streams) != 1 || p.Streams[0].Codec != "opus" || !matchesContainerFormat(p.Format.Name, "opus") {
			t.Fatalf("原始 Opus 校验失败: %+v %v", p, err)
		}
	})
}

func TestVerifyMuxedOutput_SourceTrackFailures(t *testing.T) {
	ff, err := FindFFmpeg()
	if err != nil {
		t.Skip("FFmpeg 不可用")
	}
	d := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command(ff, append([]string{"-v", "error", "-y"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg %v: %s", err, out)
		}
	}

	vH264 := filepath.Join(d, "v_h264.mp4")
	vVP9 := filepath.Join(d, "v_vp9.webm")
	aAAC := filepath.Join(d, "a_aac.m4a")
	aOpus := filepath.Join(d, "a_opus.webm")
	muxed := filepath.Join(d, "muxed.mp4")
	corrupt := filepath.Join(d, "corrupt.bin")
	missing := filepath.Join(d, "missing.mp4")

	run("-f", "lavfi", "-i", "color=s=64x64:r=10", "-t", "1", "-an", "-c:v", "libx264", vH264)
	run("-f", "lavfi", "-i", "color=s=64x64:r=10", "-t", "1", "-an", "-c:v", "libvpx-vp9", vVP9)
	run("-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "1", "-vn", "-c:a", "aac", aAAC)
	run("-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "1", "-vn", "-c:a", "libopus", aOpus)
	if err := MuxVideoAudio(context.Background(), ff, vH264, aAAC, muxed); err != nil {
		t.Fatalf("预生成合法混流文件失败: %v", err)
	}
	if err := os.WriteFile(corrupt, []byte("not-a-media-file"), 0644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		videoPath  string
		audioPath  string
		wantSubstr string
	}{
		{"missing_source_video", missing, aAAC, "源视频轨道校验失败"},
		{"corrupt_source_video", corrupt, aAAC, "源视频轨道校验失败"},
		{"missing_source_audio", vH264, missing, "源音频轨道校验失败"},
		{"corrupt_source_audio", vH264, corrupt, "源音频轨道校验失败"},
		{"source_video_without_video_stream", aAAC, aAAC, "缺少有效视频轨道编码"},
		{"source_audio_without_audio_stream", vH264, vH264, "缺少有效音频轨道编码"},
		{"source_video_codec_mismatch", vVP9, aAAC, "视频编码在封装中发生非预期变化"},
		{"source_audio_codec_mismatch", vH264, aOpus, "音频编码在封装中发生非预期变化"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyMuxedOutput(context.Background(), tc.videoPath, tc.audioPath, muxed, "mp4")
			if err == nil {
				t.Fatalf("期望返回校验错误，但得到 nil")
			}
			if !errors.Is(err, ErrMuxFailed) {
				t.Fatalf("期望错误包含 ErrMuxFailed，实际: %v", err)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("错误信息不包含 %q，实际: %v", tc.wantSubstr, err)
			}
		})
	}
}
