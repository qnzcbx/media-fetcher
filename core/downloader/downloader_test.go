package downloader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// 测试常规全量下载流程
func TestDownloader_Success(t *testing.T) {
	data := []byte("hello media streaming downloader content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	d := New(nil)
	outDir := t.TempDir()

	var reportedTotal, reportedDownloaded int64
	opts := DownloadOptions{
		OutputDir: outDir,
		Filename:  "test_file.mp4",
		OnProgress: func(downloaded, total int64) {
			reportedDownloaded = downloaded
			reportedTotal = total
		},
	}

	savedPath, err := d.Download(context.Background(), nil, server.URL, opts)
	if err != nil {
		t.Fatalf("下载失败: %v", err)
	}

	content, err := os.ReadFile(savedPath)
	if err != nil || string(content) != string(data) {
		t.Fatalf("文件内容不匹配: %v", err)
	}
	if reportedDownloaded != int64(len(data)) || reportedTotal != int64(len(data)) {
		t.Errorf("进度不匹配: downloaded=%d, total=%d", reportedDownloaded, reportedTotal)
	}

	// 验证临时 .part 文件在完成后已被清理
	matches, _ := filepath.Glob(filepath.Join(outDir, "*.part*"))
	if len(matches) > 0 {
		t.Errorf("临时文件未被清理: %v", matches)
	}
}

// H4 回归测试：Windows 保留设备名称防御（CON, PRN, AUX, NUL等）
func TestH4_WindowsReservedNames(t *testing.T) {
	reservedCases := []string{
		"CON.mp4",
		"prn.txt",
		"AUX",
		"NUL.m4s",
		"com1.bin",
		"LPT2.dat",
	}

	for _, name := range reservedCases {
		sanitized := SanitizeFilename(name)
		if sanitized == name {
			t.Errorf("未能转义 Windows 保留设备名称: %s", name)
		}
		if windowsReservedNames.MatchString(sanitized) {
			t.Errorf("转义后依然匹配 Windows 保留名称: %s -> %s", name, sanitized)
		}
	}

	// 实际下载验证：即使使用保留名也能安全下载落地而不挂起系统
	d := New(nil)
	outDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("safe data"))
	}))
	defer server.Close()

	saved, err := d.Download(context.Background(), nil, server.URL, DownloadOptions{
		OutputDir: outDir,
		Filename:  "CON.mp4",
	})
	if err != nil {
		t.Fatalf("保留名称安全落地失败: %v", err)
	}
	if filepath.Base(saved) == "CON.mp4" {
		t.Errorf("不应直接使用 CON.mp4 作为实际磁盘文件名: %s", saved)
	}
}

// H4 回归测试：并发同名下载独占临时文件，互不踩踏损坏
func TestH4_ConcurrentDownloadsNoCollision(t *testing.T) {
	d := New(nil)
	outDir := t.TempDir()

	dataA := bytes.Repeat([]byte("A"), 50000)
	dataB := bytes.Repeat([]byte("B"), 50000)

	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(dataA)))
		_, _ = w.Write(dataA)
	}))
	defer serverA.Close()

	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(dataB)))
		_, _ = w.Write(dataB)
	}))
	defer serverB.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	var errA, errB error
	var pathA, pathB string

	go func() {
		defer wg.Done()
		pathA, errA = d.Download(context.Background(), nil, serverA.URL, DownloadOptions{
			OutputDir: outDir,
			Filename:  "concurrent.mp4",
		})
	}()

	go func() {
		defer wg.Done()
		pathB, errB = d.Download(context.Background(), nil, serverB.URL, DownloadOptions{
			OutputDir: outDir,
			Filename:  "concurrent.mp4",
		})
	}()

	wg.Wait()

	if errA != nil || errB != nil {
		t.Fatalf("并发下载出错: errA=%v, errB=%v", errA, errB)
	}
	if pathA != pathB {
		t.Fatalf("路径不一致: pathA=%s, pathB=%s", pathA, pathB)
	}

	// 最终文件应当完整无损坏（内容要么是纯 A，要么是纯 B，绝不能产生交织或脏写损坏）
	finalData, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(finalData, dataA) && !bytes.Equal(finalData, dataB) {
		t.Fatal("严重缺陷：并发下载导致同名文件临时写入踩踏损坏！")
	}

	// 验证所有临时文件被彻底清理
	matches, _ := filepath.Glob(filepath.Join(outDir, "*.part*"))
	if len(matches) > 0 {
		t.Errorf("并发下载后残留临时文件: %v", matches)
	}
}

// 路径穿越防御测试
func TestDownloader_PathTraversalDefense(t *testing.T) {
	d := New(nil)
	outDir := t.TempDir()

	traversalFilenames := []string{
		"../../evil.exe",
		`..\..\windows\system32\malicious.dll`,
		"sub/../../hacked",
	}

	for _, fname := range traversalFilenames {
		opts := DownloadOptions{
			OutputDir: outDir,
			Filename:  fname,
		}
		savedPath, err := d.Download(context.Background(), nil, "https://example.com/fake", opts)
		if err == nil {
			rel, _ := filepath.Rel(outDir, savedPath)
			if filepath.IsAbs(rel) || len(rel) >= 2 && rel[:2] == ".." {
				t.Fatalf("路径穿越逃逸出 outputDir: %s -> %s", fname, savedPath)
			}
		}
	}
}

// 取消下载时临时文件清理测试
func TestDownloader_CancellationCleansUpPart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("start chunk"))
		w.(http.Flusher).Flush()
		time.Sleep(500 * time.Millisecond)
	}))
	defer server.Close()

	d := New(nil)
	outDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	opts := DownloadOptions{
		OutputDir: outDir,
		Filename:  "cancel_test.bin",
	}

	_, err := d.Download(ctx, nil, server.URL, opts)
	if err == nil {
		t.Fatal("期望收到 Context 取消错误")
	}

	targetFile := filepath.Join(outDir, "cancel_test.bin")
	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Error("取消后最终文件绝不应当存在")
	}

	matches, _ := filepath.Glob(filepath.Join(outDir, "*.part*"))
	if len(matches) > 0 {
		t.Errorf("取消后临时文件应当被清理: %v", matches)
	}
}

// M6 回归测试：Content-Range 区间不匹配时拒绝发布
func TestM6_RangeMismatchRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 客户端请求的是 10-20，服务端返回不相干的 0-5
		w.Header().Set("Content-Range", "bytes 0-5/100")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("012345"))
	}))
	defer server.Close()

	d := New(nil)
	outDir := t.TempDir()
	opts := DownloadOptions{
		OutputDir:   outDir,
		Filename:    "mismatch_range.bin",
		EnableRange: true,
		RangeStart:  10,
		HasRangeEnd: true,
		RangeEnd:    20,
	}

	_, err := d.Download(context.Background(), nil, server.URL, opts)
	if err == nil {
		t.Fatal("M6 缺陷：Range 区间与请求不符时未报错！")
	}
	if !errors.Is(err, ErrRangeMismatch) {
		t.Fatalf("期望 ErrRangeMismatch, 实际: %v", err)
	}

	// 验证未产生正式文件
	if _, err := os.Stat(filepath.Join(outDir, "mismatch_range.bin")); !os.IsNotExist(err) {
		t.Fatal("Range 不匹配时绝不能发布正式文件！")
	}
}

// M6 回归测试：传输中断或实收长度短于声明长度时拒绝发布
func TestM6_IncompleteDownloadTruncated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 声明 1000 字节，但连接只写 10 字节后断开
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("0123456789"))
		// 直接结束处理
	}))
	defer server.Close()

	d := New(nil)
	outDir := t.TempDir()
	opts := DownloadOptions{
		OutputDir: outDir,
		Filename:  "truncated.bin",
	}

	_, err := d.Download(context.Background(), nil, server.URL, opts)
	if err == nil {
		t.Fatal("M6 缺陷：实收长度不足声明长度时未报错！")
	}
	if !errors.Is(err, ErrIncompleteDownload) {
		t.Fatalf("期望 ErrIncompleteDownload, 实际: %v", err)
	}

	// 验证未生成损坏文件
	if _, err := os.Stat(filepath.Join(outDir, "truncated.bin")); !os.IsNotExist(err) {
		t.Fatal("实收不完整时绝不能发布正式文件！")
	}
}

// 测试合法 Range 下载流程
func TestDownloader_RangeDownload(t *testing.T) {
	fullData := []byte("0123456789abcdefghij")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "bytes=5-9" {
			http.Error(w, "invalid range header", 400)
			return
		}
		w.Header().Set("Content-Range", "bytes 5-9/20")
		w.Header().Set("Content-Length", "5")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(fullData[5:10])
	}))
	defer server.Close()

	d := New(nil)
	outDir := t.TempDir()
	opts := DownloadOptions{
		OutputDir:   outDir,
		Filename:    "range_chunk.bin",
		EnableRange: true,
		RangeStart:  5,
		HasRangeEnd: true,
		RangeEnd:    9,
	}

	savedPath, err := d.Download(context.Background(), nil, server.URL, opts)
	if err != nil {
		t.Fatalf("合法 Range 下载失败: %v", err)
	}

	content, err := os.ReadFile(savedPath)
	if err != nil || string(content) != "56789" {
		t.Fatalf("下载内容不匹配: got %q, want '56789'", string(content))
	}
}

// M7 回归测试：FFmpeg 混流失败或取消时绝不破坏旧文件且清理临时件
func TestM7_MuxerCancellationAndFailureCleanup(t *testing.T) {
	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "existing.mp4")

	// 预先写入一个原有的正常文件
	originalData := []byte("original_unmodified_content")
	_ = os.WriteFile(outFile, originalData, 0644)

	// 使用已超时的 context 模拟混流过程中断
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	fakeVideo := filepath.Join(outDir, "fake_video.m4s")
	fakeAudio := filepath.Join(outDir, "fake_audio.m4s")
	_ = os.WriteFile(fakeVideo, []byte("fake"), 0644)
	_ = os.WriteFile(fakeAudio, []byte("fake"), 0644)

	ffmpegPath, err := FindFFmpeg()
	if err != nil {
		t.Skip("跳过测试：未找到本地 ffmpeg")
	}

	err = MuxVideoAudio(ctx, ffmpegPath, fakeVideo, fakeAudio, outFile)
	if err == nil {
		t.Fatal("期望混流取消报错")
	}

	// 验证原有的 existing.mp4 完好无损
	currentData, err := os.ReadFile(outFile)
	if err != nil || !bytes.Equal(currentData, originalData) {
		t.Fatal("M7 缺陷：FFmpeg 失败时覆盖或损坏了原有的目标文件！")
	}

	// 验证临时 .tmp_mux 文件被彻底清理
	matches, _ := filepath.Glob(filepath.Join(outDir, ".tmp_mux*"))
	if len(matches) > 0 {
		t.Errorf("M7 缺陷：FFmpeg 失败后残留了临时文件: %v", matches)
	}
}

// 真实 FFmpeg 集成测试（支持环境变量指定素材路径，无素材时平稳跳过，不要求上传）
func TestIntegration_MuxVideoAudio_RealFFmpeg(t *testing.T) {
	// 1. 优先从环境变量读取素材路径
	videoM4s := os.Getenv("MEDIA_FETCHER_TEST_VIDEO")
	audioM4s := os.Getenv("MEDIA_FETCHER_TEST_AUDIO")
	refDir := os.Getenv("MEDIA_FETCHER_TEST_REF_DIR")

	if refDir != "" {
		if videoM4s == "" {
			videoM4s = filepath.Join(refDir, "video.m4s")
		}
		if audioM4s == "" {
			audioM4s = filepath.Join(refDir, "audio.m4s")
		}
	}

	// 2. 回退到本地默认候选路径
	if videoM4s == "" || audioM4s == "" {
		candidates := []string{
			filepath.Join("..", "..", "references", "lofter_test", "bill-downloads"),
			filepath.Join("..", "..", "tasks", "references", "lofter_test", "bill-downloads"),
		}
		for _, dir := range candidates {
			v := filepath.Join(dir, "video.m4s")
			a := filepath.Join(dir, "audio.m4s")
			if _, errV := os.Stat(v); errV == nil {
				if _, errA := os.Stat(a); errA == nil {
					videoM4s = v
					audioM4s = a
					break
				}
			}
		}
	}

	if videoM4s == "" || audioM4s == "" {
		t.Skip("跳过真实 FFmpeg 集成测试：未找到本地测试音视频素材（可通过 MEDIA_FETCHER_TEST_REF_DIR 指定，不强制要求上传素材）")
	}

	if _, err := os.Stat(videoM4s); os.IsNotExist(err) {
		t.Skipf("跳过测试：视频源文件不存在: %s", videoM4s)
	}
	if _, err := os.Stat(audioM4s); os.IsNotExist(err) {
		t.Skipf("跳过测试：音频源文件不存在: %s", audioM4s)
	}

	ffmpegPath, err := FindFFmpeg()
	if err != nil {
		t.Skipf("跳过测试：未找到系统 FFmpeg: %v", err)
	}

	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "test_muxed.mp4")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = MuxVideoAudio(ctx, ffmpegPath, videoM4s, audioM4s, outFile)
	if err != nil {
		t.Fatalf("MuxVideoAudio 真实集成测试失败: %v", err)
	}

	stat, err := os.Stat(outFile)
	if err != nil || stat.Size() == 0 {
		t.Fatalf("混流生成的 MP4 不存在或为空, 大小: %v, err: %v", stat.Size(), err)
	}
}
