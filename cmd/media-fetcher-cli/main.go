package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mdp/qrterminal/v3"

	"media-fetcher/core/auth"
	"media-fetcher/core/downloader"
	"media-fetcher/core/extractor"
	"media-fetcher/core/extractor/bilibili"
	"media-fetcher/core/extractor/lofter"
	"media-fetcher/pkg/fsutil"
	"media-fetcher/pkg/vault"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := os.Args[1]
	switch cmd {
	case "extract":
		handleExtract(os.Args[2:])
	case "download":
		handleDownload(os.Args[2:])
	case "accounts":
		handleAccounts(os.Args[2:])
	case "bili-qr":
		handleBiliQR(os.Args[2:])
	case "version":
		fmt.Println("media-fetcher core v1.0.0 (Windows Native DPAPI, Go 1.27+)")
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Media Fetcher CLI — 跨站点媒体解析与凭证中心

用法:
  media-fetcher-cli extract [-account ID] <URL>     解析媒体资源信息（音画轨道/图片列表）
  media-fetcher-cli download [选项] <URL>           下载媒体资源
  media-fetcher-cli accounts                        查看已保存的站点账号列表（脱敏）
  media-fetcher-cli bili-qr                         申请 Bilibili 扫码登录会话
  media-fetcher-cli version                         查看版本信息`)
}

func getRegistry(credService *auth.CredentialService) *extractor.Registry {
	reg := extractor.NewRegistry()
	reg.Register(bilibili.New(credService))
	reg.Register(lofter.New(credService))
	return reg
}

func handleExtract(args []string) {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	accountID := fs.String("account", "", "使用的账号ID（留空使用匿名模式）")
	requireAuth := fs.Bool("require-auth", false, "强制要求登录凭证，凭证不可用时不降级")
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "错误: 请提供需要解析的链接")
		return
	}
	targetURL := fs.Arg(0)

	v := vault.NewDefaultVault()
	credService, err := auth.NewCredentialService("", v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化凭证服务失败: %v\n", err)
		return
	}

	reg := getRegistry(credService)
	ext, err := reg.Find(targetURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "匹配站点解析器失败: %v\n", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("正在使用 [%s] 解析器解析: %s\n", ext.SiteID(), targetURL)
	media, err := ext.Extract(ctx, targetURL, extractor.ExtractOptions{
		AccountID:   *accountID,
		RequireAuth: *requireAuth,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析失败: %v\n", err)
		return
	}

	fmt.Println("=========================================")
	fmt.Printf("站点: %s\n", media.SiteID)
	fmt.Printf("ID:   %s\n", media.ID)
	fmt.Printf("标题: %s\n", media.Title)
	if media.Author != "" {
		fmt.Printf("作者: %s\n", media.Author)
	}
	fmt.Printf("模式: 匿名=%v (使用账号: %s)\n", media.IsAnonymous, media.UsedAccountID)

	if len(media.VideoStreams) > 0 {
		fmt.Printf("\n可用视频流 (%d 条):\n", len(media.VideoStreams))
		for i, v := range media.VideoStreams {
			authNote := ""
			if v.NeedAuth {
				authNote = " [需登录]"
			}
			fmt.Printf("  [%d] %s %dx%d (codec=%s, 码率=%d bps)%s\n",
				i, v.QualityLabel, v.Width, v.Height, v.Codec, v.Bandwidth, authNote)
		}
	}

	if len(media.AudioStreams) > 0 {
		fmt.Printf("\n可用音轨 (%d 条):\n", len(media.AudioStreams))
		for i, a := range media.AudioStreams {
			fmt.Printf("  [%d] %s (codec=%s, 码率=%d bps)\n", i, a.Quality, a.Codec, a.Bandwidth)
		}
	}

	if len(media.Images) > 0 {
		fmt.Printf("\n可用图片列表 (%d 张):\n", len(media.Images))
		for _, img := range media.Images {
			preview := img.URL
			if len(preview) > 80 {
				preview = preview[:77] + "..."
			}
			fmt.Printf("  [%03d] %s\n", img.Index, preview)
		}
	}
	fmt.Println("=========================================")
}

func handleAccounts(args []string) {
	v := vault.NewDefaultVault()
	credService, err := auth.NewCredentialService("", v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化凭证服务失败: %v\n", err)
		return
	}

	accounts := credService.ListAccounts("")
	if len(accounts) == 0 {
		fmt.Println("当前没有任何已保存的站点账号。")
		return
	}

	fmt.Printf("已配置的站点账号列表 (%d 个):\n", len(accounts))
	for _, a := range accounts {
		activeMarker := " "
		if a.IsActive {
			activeMarker = "*"
		}
		fmt.Printf("[%s] 平台: %-10s 账号ID: %-12s 昵称: %-15s 状态: %s\n",
			activeMarker, a.SiteID, a.AccountID, a.Name, a.Status)
	}
}

func handleBiliQR(args []string) {
	fs := flag.NewFlagSet("bili-qr", flag.ExitOnError)
	debug := fs.Bool("debug", false, "输出调试信息")
	_ = fs.Parse(args)

	v := vault.NewDefaultVault()
	credService, err := auth.NewCredentialService("", v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化凭证服务失败: %v\n", err)
		return
	}

	coord := auth.NewBilibiliQRLoginCoordinator(credService)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sess, err := coord.GenerateQRCode(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "申请二维码失败: %v\n", err)
		return
	}

	fmt.Println("已成功申请 Bilibili 扫码登录:")
	fmt.Printf("  扫码链接: %s\n", sess.URL)
	fmt.Println("  请使用 Bilibili 手机客户端扫描以下二维码：")
	qrterminal.GenerateHalfBlock(sess.URL, qrterminal.L, os.Stdout)

	ctxPoll, cancelPoll := context.WithCancel(context.Background())
	defer cancelPoll()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		fmt.Println("\n检测到中断信号，正在取消...")
		cancelPoll()
	}()

	fmt.Println("\n开始轮询扫码状态...")
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	lastState := auth.QRStateWaiting
	for {
		select {
		case <-ctxPoll.Done():
			fmt.Println("\n已退出登录流程。")
			return
		case <-ticker.C:
			res, err := coord.PollStatus(ctxPoll, sess.QRCodeKey)
			if err != nil {
				fmt.Printf("\n轮询状态错误: %v", err)
				continue
			}
			if *debug {
				fmt.Printf("\n[DEBUG] RawCode: %d, State: %s, Message: %s\n", res.RawCode, res.State, res.Message)
			}
			if res.State != lastState {
				fmt.Printf("\n[%s] %s", res.State, res.Message)
				lastState = res.State
			} else if res.State == auth.QRStateWaiting && !*debug {
				fmt.Print(".")
			}

			if res.State == auth.QRStateConfirmed {
				if res.AccountInfo != nil {
					fmt.Printf("\n登录成功！已保存账号: %s (ID: %s)\n", res.AccountInfo.Name, res.AccountInfo.AccountID)
				} else {
					fmt.Println("\n登录成功！凭证已保存。")
				}
				return
			} else if res.State == auth.QRStateExpired {
				fmt.Println("\n请重新运行命令申请新的二维码。")
				return
			} else if res.State == auth.QRStateFailed {
				fmt.Println("\n扫码流程失败，请重试。")
				return
			}
		}
	}
}

func handleDownload(args []string) {
	fs := flag.NewFlagSet("download", flag.ExitOnError)
	outputDir := fs.String("output", ".", "输出目录")
	accountID := fs.String("account", "", "指定登录账号ID")
	videoIdx := fs.Int("video", -1, "选择视频流序号")
	audioIdx := fs.Int("audio", -1, "选择音频流序号")
	audioFormat := fs.String("audio-format", "original", "音频输出 original/m4a/mp3")
	audioOnly := fs.Bool("audio-only", false, "仅下载音频")
	requireAuth := fs.Bool("require-auth", false, "必须使用登录账号")
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "错误: 请提供需要下载的链接")
		return
	}
	targetURL := fs.Arg(0)

	v := vault.NewDefaultVault()
	credService, err := auth.NewCredentialService("", v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化凭证服务失败: %v\n", err)
		return
	}

	reg := getRegistry(credService)
	ext, err := reg.Find(targetURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "匹配站点解析器失败: %v\n", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		fmt.Println("\n收到中断信号，取消下载...")
		cancel()
	}()

	fmt.Printf("正在解析资源: %s\n", targetURL)
	media, err := ext.Extract(ctx, targetURL, extractor.ExtractOptions{
		AccountID:   *accountID,
		RequireAuth: *requireAuth,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析失败: %v\n", err)
		return
	}

	var client *http.Client
	if media.UsedAccountID != "" {
		client, err = credService.GetClient(media.SiteID, media.UsedAccountID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "获取凭证客户端失败: %v\n", err)
			return
		}
		fmt.Printf("使用账号 %s 登录态下载\n", media.UsedAccountID)
	} else {
		client = credService.GetAnonymousClient(media.SiteID)
		fmt.Println("使用匿名态下载")
	}

	var downloadClient *http.Client
	if client != nil {
		c := *client
		c.Timeout = 0
		downloadClient = &c
	}
	d := downloader.New(downloadClient)

	// LOFTER 图片下载逻辑
	if len(media.Images) > 0 && len(media.VideoStreams) == 0 && len(media.AudioStreams) == 0 {
		fmt.Printf("发现 %d 张图片，准备下载...\n", len(media.Images))
		for _, img := range media.Images {
			filename := fmt.Sprintf("%s_%s", media.ID, img.Filename)
			if img.Filename == "" {
				filename = fmt.Sprintf("%s_%03d.jpg", media.ID, img.Index)
			}
			opts := downloader.DownloadOptions{
				OutputDir: *outputDir,
				Filename:  filename,
			}
			fmt.Printf("下载图片 %s ... ", filename)
			var err error
			_, err = d.Download(ctx, nil, img.URL, opts)
			if err != nil && len(img.BackupURLs) > 0 {
				for _, backupURL := range img.BackupURLs {
					_, err = d.Download(ctx, nil, backupURL, opts)
					if err == nil {
						break
					}
				}
			}
			if err != nil {
				fmt.Printf("失败: %v\n", err)
			} else {
				fmt.Println("成功")
			}
		}
		return
	}

	// 视频/音频选择逻辑
	vIdx := *videoIdx
	aIdx := *audioIdx

	if !*audioOnly && len(media.VideoStreams) > 0 && vIdx < 0 {
		fmt.Printf("可用视频流:\n")
		for i, vs := range media.VideoStreams {
			fmt.Printf("  [%d] %s (codec=%s)\n", i, vs.QualityLabel, vs.Codec)
		}
		fmt.Print("请选择视频流序号 (直接回车默认 0, 输入 -1 放弃): ")
		var input string
		fmt.Scanln(&input)
		if input == "" {
			vIdx = 0
		} else {
			vIdx, _ = strconv.Atoi(input)
		}
	}

	if len(media.AudioStreams) > 0 && aIdx < 0 {
		fmt.Printf("可用音轨:\n")
		for i, as := range media.AudioStreams {
			fmt.Printf("  [%d] %s (codec=%s)\n", i, as.Quality, as.Codec)
		}
		fmt.Print("请选择音轨序号 (直接回车默认 0, 输入 -1 放弃): ")
		var input string
		fmt.Scanln(&input)
		if input == "" {
			aIdx = 0
		} else {
			aIdx, _ = strconv.Atoi(input)
		}
	}

	var downloadedVideo string
	var downloadedAudio string

	title := downloader.SanitizeFilename(media.Title)
	if title == "media_download" {
		title = media.ID
	}

	if !*audioOnly && vIdx >= 0 && vIdx < len(media.VideoStreams) {
		vs := media.VideoStreams[vIdx]
		opts := downloader.DownloadOptions{
			OutputDir: *outputDir,
			Filename:  fmt.Sprintf("%s.video.tmp", title),
			Referer:   "https://www.bilibili.com", // 如果需要可以从 MediaInfo 扩展
			OnProgress: func(downloaded, total int64) {
			},
		}
		fmt.Printf("正在下载视频: %s\n", vs.QualityLabel)

		var err error
		downloadedVideo, err = d.Download(ctx, nil, vs.URL, opts)
		if err != nil {
			if len(vs.BackupURLs) > 0 {
				fmt.Printf("视频主链接下载失败: %v, 尝试备用链接...\n", err)
				for _, backupURL := range vs.BackupURLs {
					downloadedVideo, err = d.Download(ctx, nil, backupURL, opts)
					if err == nil {
						break
					}
				}
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "视频下载失败: %v\n", err)
				return
			}
		}
	}

	if aIdx >= 0 && aIdx < len(media.AudioStreams) {
		as := media.AudioStreams[aIdx]
		ext := ".source.tmp"
		opts := downloader.DownloadOptions{
			OutputDir: *outputDir,
			Filename:  fmt.Sprintf("%s.audio%s", title, ext),
			Referer:   "https://www.bilibili.com",
		}
		if !*audioOnly && downloadedVideo != "" {
			opts.Filename = fmt.Sprintf("%s.audio.tmp", title)
		}

		fmt.Printf("正在下载音轨: %s\n", as.Quality)
		var err error
		downloadedAudio, err = d.Download(ctx, nil, as.URL, opts)
		if err != nil {
			if len(as.BackupURLs) > 0 {
				fmt.Printf("音频主链接下载失败: %v, 尝试备用链接...\n", err)
				for _, backupURL := range as.BackupURLs {
					downloadedAudio, err = d.Download(ctx, nil, backupURL, opts)
					if err == nil {
						break
					}
				}
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "音频下载失败: %v\n", err)
				return
			}
		}
	}

	if downloadedVideo != "" && downloadedAudio != "" {
		fmt.Println("正在合并音视频...")
		ffmpegPath, err := downloader.FindFFmpeg()
		if err != nil {
			fmt.Fprintf(os.Stderr, "合并失败: 未找到 FFmpeg (请配置环境变量或将其放置于系统目录中): %v\n", err)
			return
		}
		finalOut := filepath.Join(*outputDir, fmt.Sprintf("%s.mp4", title))
		err = downloader.MuxVideoAudio(ctx, ffmpegPath, downloadedVideo, downloadedAudio, finalOut)
		if err != nil {
			fmt.Fprintf(os.Stderr, "合并失败: %v\n", err)
		} else {
			fmt.Printf("合并完成: %s\n", finalOut)
			os.Remove(downloadedVideo)
			os.Remove(downloadedAudio)
		}
	} else if downloadedVideo != "" {
		finalOut := filepath.Join(*outputDir, fmt.Sprintf("%s.mp4", title))
		work, err := os.MkdirTemp(*outputDir, ".video-output-")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		defer os.RemoveAll(work)
		prepared, err := downloader.PrepareSingleTrack(ctx, downloadedVideo, filepath.Join(work, "output.mp4"), "mp4", func(stage string) { fmt.Println(stage) })
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if err := fsutil.PublishFile(prepared, finalOut, fsutil.StrategyCreateNew); err != nil {
			fmt.Fprintf(os.Stderr, "文件发布失败: %v\n", err)
		} else {
			fmt.Printf("下载完成: %s\n", finalOut)
		}
	} else if downloadedAudio != "" {
		format := *audioFormat
		var err error
		if format == "original" {
			format, err = downloader.OriginalAudioFormat(ctx, downloadedAudio)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		work, err := os.MkdirTemp(*outputDir, ".audio-output-")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		defer os.RemoveAll(work)
		staged, err := downloader.PrepareSingleTrack(ctx, downloadedAudio, filepath.Join(work, "output."+format), format, func(stage string) { fmt.Println(stage) })
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		final := filepath.Join(*outputDir, title+"."+format)
		if err = fsutil.PublishFile(staged, final, fsutil.StrategyCreateNew); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		os.Remove(downloadedAudio)
		fmt.Printf("下载完成: %s\n", final)
	} else {
		fmt.Println("未下载任何内容。")
	}
}

var _ = json.Marshal
var _ = downloader.New
var _ = strings.TrimSpace
