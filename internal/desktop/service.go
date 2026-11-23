package desktop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"media-fetcher/core/auth"
	"media-fetcher/core/downloader"
	"media-fetcher/core/extractor"
	"media-fetcher/core/extractor/bilibili"
	"media-fetcher/core/extractor/lofter"
	"media-fetcher/pkg/fsutil"
	"media-fetcher/pkg/vault"
)

type DesktopService struct {
	credService *auth.CredentialService
	registry    *extractor.Registry
	win         *mygo.Window

	// 下载状态
	downloadMu     sync.Mutex
	cancelDownload context.CancelFunc
	activeDownload bool
}

func NewDesktopService() (*DesktopService, error) {
	v := vault.NewDefaultVault()
	credService, err := auth.NewCredentialService("", v)
	if err != nil {
		return nil, fmt.Errorf("init cred service failed: %v", err)
	}

	reg := extractor.NewRegistry()
	reg.Register(bilibili.New(credService))
	reg.Register(lofter.New(credService))

	return &DesktopService{
		credService: credService,
		registry:    reg,
	}, nil
}

func (s *DesktopService) SetWindow(w *mygo.Window) {
	s.win = w
}

// MediaInfoResponse 发送给前端的媒体信息响应结构体
type MediaInfoResponse struct {
	SiteID        string                  `json:"siteId"`
	ID            string                  `json:"id"`
	Title         string                  `json:"title"`
	Author        string                  `json:"author"`
	CoverURL      string                  `json:"coverUrl"`
	IsAnonymous   bool                    `json:"isAnonymous"`
	UsedAccountID string                  `json:"usedAccountId"`
	VideoStreams  []extractor.VideoStream `json:"videoStreams"`
	AudioStreams  []extractor.AudioStream `json:"audioStreams"`
	Images        []extractor.ImageItem       `json:"images"`
}

func (s *DesktopService) Extract(urlStr string, accountID string) (MediaInfoResponse, error) {
	ext, err := s.registry.Find(urlStr)
	if err != nil {
		return MediaInfoResponse{}, fmt.Errorf("找不到对应的解析器: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	media, err := ext.Extract(ctx, urlStr, extractor.ExtractOptions{
		AccountID:   accountID,
		RequireAuth: false,
	})
	if err != nil {
		return MediaInfoResponse{}, fmt.Errorf("解析失败: %v", err)
	}

	return MediaInfoResponse{
		SiteID:        media.SiteID,
		ID:            media.ID,
		Title:         media.Title,
		Author:        media.Author,
		CoverURL:      media.CoverURL,
		IsAnonymous:   media.IsAnonymous,
		UsedAccountID: media.UsedAccountID,
		VideoStreams:  media.VideoStreams,
		AudioStreams:  media.AudioStreams,
		Images:        media.Images,
	}, nil
}

type SafeAccount struct {
	SiteID    string `json:"siteId"`
	AccountID string `json:"accountId"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	Status    string `json:"status"`
	IsActive  bool   `json:"isActive"`
}

func (s *DesktopService) GetAccounts() []SafeAccount {
	accounts := s.credService.ListAccounts("")
	var res []SafeAccount
	for _, a := range accounts {
		res = append(res, SafeAccount{
			SiteID:    a.SiteID,
			AccountID: a.AccountID,
			Name:      a.Name,
			AvatarURL: a.AvatarURL,
			Status:    string(a.Status),
			IsActive:  a.IsActive,
		})
	}
	return res
}

// BiliQRSession Bilibili 前端二维码会话信息
type BiliQRSession struct {
	QRCodeKey string `json:"qrcodeKey"`
	URL       string `json:"url"`
}

func (s *DesktopService) GenerateBiliQRCode() (BiliQRSession, error) {
	coord := auth.NewBilibiliQRLoginCoordinator(s.credService)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sess, err := coord.GenerateQRCode(ctx)
	if err != nil {
		return BiliQRSession{}, err
	}
	return BiliQRSession{
		QRCodeKey: sess.QRCodeKey,
		URL:       sess.URL,
	}, nil
}

type PollResult struct {
	State   string       `json:"state"`
	Message string       `json:"message"`
	Account *SafeAccount `json:"account,omitempty"`
}

func (s *DesktopService) PollBiliQRCode(qrcodeKey string) (PollResult, error) {
	coord := auth.NewBilibiliQRLoginCoordinator(s.credService)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := coord.PollStatus(ctx, qrcodeKey)
	if err != nil {
		return PollResult{}, err
	}

	var acc *SafeAccount
	if res.AccountInfo != nil {
		acc = &SafeAccount{
			SiteID:    res.AccountInfo.SiteID,
			AccountID: res.AccountInfo.AccountID,
			Name:      res.AccountInfo.Name,
			AvatarURL: res.AccountInfo.AvatarURL,
			Status:    string(res.AccountInfo.Status),
			IsActive:  res.AccountInfo.IsActive,
		}
	}

	return PollResult{
		State:   string(res.State),
		Message: res.Message,
		Account: acc,
	}, nil
}

type DownloadParams struct {
	URL        string
	AccountID  string
	OutputDir  string
	VideoID    int
	VideoCodec string
	AudioID    int
}

var ProgressEvent = mygo.NewEvent[ProgressInfo]("progress")

type ProgressInfo struct {
	Stage      string `json:"stage"` // "video", "audio", "muxing", "done", "error"
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Message    string `json:"message"`
	FinalPath  string `json:"finalPath"`
}

func (s *DesktopService) StartDownload(params DownloadParams) error {
	s.downloadMu.Lock()
	if s.activeDownload {
		s.downloadMu.Unlock()
		return fmt.Errorf("当前已有任务正在下载或取消中，请稍后再试")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelDownload = cancel
	s.activeDownload = true
	s.downloadMu.Unlock()

	go func() {
		defer func() {
			s.downloadMu.Lock()
			s.cancelDownload = nil
			s.activeDownload = false
			s.downloadMu.Unlock()
		}()
		err := s.doDownload(ctx, params)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				// 用户取消，不当作普通下载失败覆盖前端状态
			} else {
				ProgressEvent.Emit(s.win, ProgressInfo{Stage: "error", Message: err.Error()})
			}
		}
	}()
	return nil
}

func (s *DesktopService) CancelDownload() {
	s.downloadMu.Lock()
	defer s.downloadMu.Unlock()
	if s.cancelDownload != nil {
		s.cancelDownload()
	}
}

func safePublish(src, dst string) error {
	err := fsutil.PublishFile(src, dst, fsutil.StrategyReplace)
	if err == nil {
		return nil
	}

	// 跨卷发布：复制到目标同目录下的临时文件，再同卷原子发布
	dstDir := filepath.Dir(dst)
	tempDst, err2 := os.CreateTemp(dstDir, ".publish-tmp-*")
	if err2 != nil {
		return err
	}
	tempName := tempDst.Name()
	defer os.Remove(tempName)

	srcFile, err2 := os.Open(src)
	if err2 != nil {
		tempDst.Close()
		return err
	}
	defer srcFile.Close()

	_, err2 = io.Copy(tempDst, srcFile)
	tempDst.Close()
	if err2 != nil {
		return err
	}

	return fsutil.PublishFile(tempName, dst, fsutil.StrategyReplace)
}

func (s *DesktopService) doDownload(ctx context.Context, params DownloadParams) error {
	ext, err := s.registry.Find(params.URL)
	if err != nil {
		return fmt.Errorf("找不到解析器: %v", err)
	}

	media, err := ext.Extract(ctx, params.URL, extractor.ExtractOptions{
		AccountID: params.AccountID,
	})
	if err != nil {
		return fmt.Errorf("解析失败: %v", err)
	}

	client := s.credService.GetAnonymousClient(media.SiteID)
	if media.UsedAccountID != "" {
		if c, err := s.credService.GetClient(media.SiteID, media.UsedAccountID); err == nil {
			client = c
		}
	}

	if client != nil {
		c := *client
		c.Timeout = 0
		client = &c
	}
	d := downloader.New(client)

	outputDir := params.OutputDir
	if outputDir == "" {
		home, _ := os.UserHomeDir()
		outputDir = filepath.Join(home, "Downloads")
	}

	// LOFTER 图片下载逻辑
	if len(media.Images) > 0 && len(media.VideoStreams) == 0 && len(media.AudioStreams) == 0 {
		ProgressEvent.Emit(s.win, ProgressInfo{Stage: "images", Message: fmt.Sprintf("准备下载 %d 张图片", len(media.Images))})
		for i, img := range media.Images {
			filename := fmt.Sprintf("%s_%03d%s", media.ID, img.Index, filepath.Ext(img.URL))
			opts := downloader.DownloadOptions{
				OutputDir: outputDir,
				Filename:  filename,
				OnProgress: func(downloaded, total int64) {
					ProgressEvent.Emit(s.win, ProgressInfo{
						Stage:      "images",
						Downloaded: downloaded,
						Total:      total,
						Message:    fmt.Sprintf("下载进度: %d/%d", i+1, len(media.Images)),
					})
				},
			}
			_, err := d.Download(ctx, nil, img.URL, opts)
			if err != nil {
				return err
			}
		}
		ProgressEvent.Emit(s.win, ProgressInfo{Stage: "done", Message: "图片下载完成", FinalPath: outputDir})
		return nil
	}

	// 创建独立临时工作目录
	workDir, err := os.MkdirTemp("", "media-fetcher-work-*")
	if err != nil {
		return fmt.Errorf("创建临时工作目录失败: %v", err)
	}
	defer os.RemoveAll(workDir)

	var downloadedVideo, downloadedAudio string
	title := downloader.SanitizeFilename(media.Title)
	if title == "media_download" {
		title = media.ID
	}

	var targetVideo *extractor.VideoStream
	var targetAudio *extractor.AudioStream

	if params.VideoID >= 0 {
		for _, vs := range media.VideoStreams {
			if vs.ID == params.VideoID && vs.Codec == params.VideoCodec {
				targetVideo = &vs
				break
			}
		}
		if targetVideo == nil {
			return fmt.Errorf("所选视频画质或编码已失效，请重新解析")
		}
	}

	if params.AudioID >= 0 {
		for _, as := range media.AudioStreams {
			if as.ID == params.AudioID {
				targetAudio = &as
				break
			}
		}
		if targetAudio == nil {
			return fmt.Errorf("所选音质已失效，请重新解析")
		}
	}

	if targetVideo != nil {
		opts := downloader.DownloadOptions{
			OutputDir: workDir,
			Filename:  "video.tmp",
			Referer:   "https://www.bilibili.com",
			OnProgress: func(downloaded, total int64) {
				ProgressEvent.Emit(s.win, ProgressInfo{
					Stage:      "video",
					Downloaded: downloaded,
					Total:      total,
					Message:    "正在下载视频",
				})
			},
		}

		var dlErr error
		downloadedVideo, dlErr = d.Download(ctx, nil, targetVideo.URL, opts)
		if dlErr != nil {
			if len(targetVideo.BackupURLs) > 0 {
				for _, backupURL := range targetVideo.BackupURLs {
					downloadedVideo, dlErr = d.Download(ctx, nil, backupURL, opts)
					if dlErr == nil {
						break
					}
				}
			}
			if dlErr != nil {
				return fmt.Errorf("视频下载失败: %v", dlErr)
			}
		}
	}

	if targetAudio != nil {
		opts := downloader.DownloadOptions{
			OutputDir: workDir,
			Filename:  "audio.tmp",
			Referer:   "https://www.bilibili.com",
			OnProgress: func(downloaded, total int64) {
				ProgressEvent.Emit(s.win, ProgressInfo{
					Stage:      "audio",
					Downloaded: downloaded,
					Total:      total,
					Message:    "正在下载音轨",
				})
			},
		}
		var dlErr error
		downloadedAudio, dlErr = d.Download(ctx, nil, targetAudio.URL, opts)
		if dlErr != nil {
			if len(targetAudio.BackupURLs) > 0 {
				for _, backupURL := range targetAudio.BackupURLs {
					downloadedAudio, dlErr = d.Download(ctx, nil, backupURL, opts)
					if dlErr == nil {
						break
					}
				}
			}
			if dlErr != nil {
				return fmt.Errorf("音频下载失败: %v", dlErr)
			}
		}
	}

	if downloadedVideo != "" && downloadedAudio != "" {
		ProgressEvent.Emit(s.win, ProgressInfo{Stage: "muxing", Message: "正在合并音视频..."})
		ffmpegPath, err := downloader.FindFFmpeg()
		if err != nil {
			return fmt.Errorf("合并失败: 未找到 FFmpeg: %v", err)
		}
		finalOut := filepath.Join(outputDir, fmt.Sprintf("%s.mp4", title))
		err = downloader.MuxVideoAudio(ctx, ffmpegPath, downloadedVideo, downloadedAudio, finalOut)
		if err != nil {
			return err
		}
		ProgressEvent.Emit(s.win, ProgressInfo{Stage: "done", Message: "下载并合并完成", FinalPath: finalOut})
	} else if downloadedVideo != "" {
		finalOut := filepath.Join(outputDir, fmt.Sprintf("%s.mp4", title))
		if err := safePublish(downloadedVideo, finalOut); err != nil {
			return fmt.Errorf("文件发布失败: %v", err)
		}
		ProgressEvent.Emit(s.win, ProgressInfo{Stage: "done", Message: "视频下载完成", FinalPath: finalOut})
	} else if downloadedAudio != "" {
		finalOut := filepath.Join(outputDir, fmt.Sprintf("%s.m4a", title))
		if err := safePublish(downloadedAudio, finalOut); err != nil {
			return fmt.Errorf("文件发布失败: %v", err)
		}
		ProgressEvent.Emit(s.win, ProgressInfo{Stage: "done", Message: "音频下载完成", FinalPath: finalOut})
	} else {
		return fmt.Errorf("未下载任何内容")
	}

	return nil
}

func (s *DesktopService) SelectDirectory() (string, error) {
	// 目前 MyGo 暂未在文档中提供开箱即用的原生目录选择弹窗 API
	// 作为安全和克制的选择，暂时回退默认而不引入未受官方支持的包。
	return "", fmt.Errorf("not implemented")
}
