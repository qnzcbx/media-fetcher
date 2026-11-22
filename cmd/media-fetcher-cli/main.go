package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"media-fetcher/core/auth"
	"media-fetcher/core/downloader"
	"media-fetcher/core/extractor"
	"media-fetcher/core/extractor/bilibili"
	"media-fetcher/core/extractor/lofter"
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
  media-fetcher-cli accounts                        查看已保存的站点账号列表（脱敏）
  media-fetcher-cli bili-qr                         申请 Bilibili 扫码登录会话
  media-fetcher-cli version                         查看版本信息`)
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

	reg := extractor.NewRegistry()
	reg.Register(bilibili.New(credService))
	reg.Register(lofter.New(credService))

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
		for _, v := range media.VideoStreams {
			authNote := ""
			if v.NeedAuth {
				authNote = " [需登录/大会员]"
			}
			fmt.Printf("  - [%s] %dx%d (codec=%s, 码率=%d bps)%s\n",
				v.QualityLabel, v.Width, v.Height, v.Codec, v.Bandwidth, authNote)
		}
	}

	if len(media.AudioStreams) > 0 {
		fmt.Printf("\n可用音轨 (%d 条):\n", len(media.AudioStreams))
		for _, a := range media.AudioStreams {
			fmt.Printf("  - [%s] (codec=%s, 码率=%d bps)\n", a.Quality, a.Codec, a.Bandwidth)
		}
	}

	if len(media.Images) > 0 {
		fmt.Printf("\n可用图片列表 (%d 张):\n", len(media.Images))
		for _, img := range media.Images {
			// URL 脱敏显示前缀
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
	fmt.Printf("  会话密钥: %s\n", sess.QRCodeKey)
	fmt.Println("提示: 请使用手机 Bilibili 客户端扫描上方链接二维码。")
}

var _ = json.Marshal
var _ = downloader.New
var _ = strings.TrimSpace
