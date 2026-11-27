package downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOriginalMediaOutputs(t *testing.T) {
	ff, err := FindFFmpeg()
	if err != nil {
		t.Skip(err)
	}
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command(ff, append([]string{"-v", "error", "-y"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg %v: %s", err, out)
		}
	}
	pcm := func(path string) []byte {
		t.Helper()
		out, err := exec.Command(ff, "-v", "error", "-i", path, "-map", "0:a:0", "-f", "s16le", "-acodec", "pcm_s16le", "-").Output()
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	for _, tc := range []struct{ codec, container, extension string }{{"aac", "mp4", "m4a"}, {"flac", "mp4", "flac"}, {"flac", "flac", "flac"}, {"eac3", "eac3", "eac3"}, {"ac3", "ac3", "ac3"}, {"libopus", "ogg", "opus"}, {"libmp3lame", "mp3", "mp3"}} {
		t.Run(tc.codec+tc.container, func(t *testing.T) {
			d := t.TempDir()
			input := filepath.Join(d, "input.bin")
			run("-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "1", "-c:a", tc.codec, "-strict", "-2", "-f", tc.container, input)
			format, err := OriginalAudioFormat(context.Background(), input)
			if err != nil || format != tc.extension {
				t.Fatalf("%s %v", format, err)
			}
			output, err := PrepareSingleTrack(context.Background(), input, filepath.Join(d, "out."+format), format, func(stage string) {
				if stage == "transcoding" {
					t.Fatal("原始模式进行了转码")
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(pcm(input), pcm(output)) {
				t.Fatal("PCM 数据改变")
			}
			video := filepath.Join(d, "video.mp4")
			run("-f", "lavfi", "-i", "color=s=32x32:r=10", "-t", "1", "-an", "-c:v", "libx264", video)
			muxed := filepath.Join(d, "mux.mp4")
			if err := MuxVideoAudio(t.Context(), ff, video, input, muxed); err != nil {
				t.Fatal(err)
			}
			probe, err := ProbeMedia(t.Context(), muxed)
			if err != nil || len(probe.Streams) != 2 {
				t.Fatal("合并轨道无效")
			}
			sourceProbe, _ := ProbeMedia(t.Context(), input)
			if probe.Streams[0].Codec != "h264" || probe.Streams[1].Codec != sourceProbe.Streams[0].Codec {
				t.Fatal("合并改变了源编码")
			}
			for _, format := range []string{"m4a", "mp3"} {
				path, err := PrepareSingleTrack(t.Context(), input, filepath.Join(d, "convert."+format), format, func(string) {})
				if err != nil {
					t.Fatal(err)
				}
				probe, err := ProbeMedia(t.Context(), path)
				if err != nil || len(probe.Streams) != 1 || probe.Seconds() <= 0 {
					t.Fatal("输出无效")
				}
			}
		})
	}
	if _, err := OriginalAudioFormat(t.Context(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("探测失败必须拒绝")
	}
}

func TestOutputPolicyRejectsMismatches(t *testing.T) {
	var p MediaProbe
	p.Format.Name = "flac"
	if matchesOutput(p, "m4a", "aac") {
		t.Fatal("无轨道被接受")
	}
	json.Unmarshal([]byte(`{"format":{"format_name":"flac"},"streams":[{"codec_type":"audio","codec_name":"flac"}]}`), &p)
	if matchesOutput(p, "m4a", "flac") || matchesOutput(p, "mp4", "flac") {
		t.Fatal("错误容器被接受")
	}
	if !matchesOutput(p, "flac", "flac") {
		t.Fatal("正确容器被拒绝")
	}
}
