package desktop

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"media-fetcher/core/auth"
	"media-fetcher/core/downloader"
	"media-fetcher/core/extractor"
	"media-fetcher/core/extractor/bilibili"
	"media-fetcher/core/extractor/lofter"
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
	// 预览缓存
	previewCache sync.Map
	previewMu    sync.Mutex
	previewKeys  []string
	historyDir   string
	historyMu    sync.Mutex
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

	historyDir, err := historyDirectory()
	if err != nil {
		return nil, err
	}
	return &DesktopService{
		historyDir:  historyDir,
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
	Images        []extractor.ImageItem   `json:"images"`
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

func (s *DesktopService) FetchPreviewBase64(imgUrl string, siteId string) (string, error) {
	if val, ok := s.previewCache.Load(imgUrl); ok {
		return val.(string), nil
	}

	targetURL := imgUrl
	if siteId == "lofter" {
		u, err := url.Parse(imgUrl)
		if err == nil {
			q := u.Query()
			q.Set("imageView", "")
			q.Set("thumbnail", "400x0")
			u.RawQuery = q.Encode()
			targetURL = u.String()
		}
	} else if siteId == "bilibili" {
		if !strings.Contains(imgUrl, "@") {
			targetURL = imgUrl + "@400w.jpg"
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	if siteId == "lofter" {
		req.Header.Set("Referer", "https://www.lofter.com")
	} else if siteId == "bilibili" {
		req.Header.Set("Referer", "https://www.bilibili.com")
	}

	client := s.credService.GetAnonymousClient(siteId)
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil {
		return "", err
	}

	if len(b) > 2<<20 {
		return "", errors.New("预览图片过大")
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	b64 := fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(b))
	s.previewMu.Lock()
	if len(s.previewKeys) >= 32 {
		s.previewCache.Delete(s.previewKeys[0])
		s.previewKeys = s.previewKeys[1:]
	}
	if _, ok := s.previewCache.Load(imgUrl); !ok {
		s.previewKeys = append(s.previewKeys, imgUrl)
	}
	s.previewCache.Store(imgUrl, b64)
	s.previewMu.Unlock()
	return b64, nil
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
	Mode          string
	AudioFormat   string
	PreviewURLs   []string
	CoverPreview  string
	previews      map[string]string
	TaskID        string
	URL           string
	AccountID     string
	OutputDir     string
	VideoID       int
	VideoCodec    string
	AudioID       int
	SelectedImage []int
}

var ProgressEvent = mygo.NewEvent[ProgressInfo]("progress")

type ProgressInfo struct {
	TaskID     string          `json:"taskId"`
	ItemID     string          `json:"itemId"`
	Record     *DownloadRecord `json:"record,omitempty"`
	Stage      string          `json:"stage"` // "video", "audio", "muxing", "done", "error"
	Downloaded int64           `json:"downloaded"`
	Total      int64           `json:"total"`
	Message    string          `json:"message"`
	FinalPath  string          `json:"finalPath"`
}

func (s *DesktopService) StartDownload(params DownloadParams) error {
	directory, err := s.ValidateDownloadDirectory(params.OutputDir)
	if err != nil {
		return err
	}
	params.OutputDir = directory
	if err := validateDownload(params); err != nil {
		return err
	}
	if len(params.CoverPreview) > base64.StdEncoding.EncodedLen(2<<20)+128 {
		params.CoverPreview = ""
	}
	previews := make(map[string]string)
	for _, key := range params.PreviewURLs[:min(len(params.PreviewURLs), 32)] {
		if value, ok := s.previewCache.Load(key); ok {
			previews[key] = value.(string)
		}
	}
	params.previews = previews
	s.downloadMu.Lock()
	if s.activeDownload {
		s.downloadMu.Unlock()
		return fmt.Errorf("当前已有任务正在下载或取消中，请稍后再试")
	}

	if params.TaskID != "" && !taskIDPattern.MatchString(params.TaskID) {
		s.downloadMu.Unlock()
		return errors.New("无效的下载任务标识")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelDownload = cancel
	s.activeDownload = true
	s.downloadMu.Unlock()

	go func() {
		err := s.doDownload(ctx, params)
		stage, message := "finished", ""
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				stage = "cancelled"
			} else {
				stage = "error"
				message = "下载失败，请重试"
				var outputErr *outputFailure
				if errors.As(err, &outputErr) {
					message = outputErr.Error()
				}
			}
		}
		cancel()
		s.downloadMu.Lock()
		s.cancelDownload = nil
		s.activeDownload = false
		s.emitDownload(params.TaskID, "", ProgressInfo{Stage: stage, Message: message})
		s.downloadMu.Unlock()
	}()

	return nil
}

func (s *DesktopService) emitDownload(taskID, itemID string, info ProgressInfo) {
	if s.win == nil {
		return
	}
	info.TaskID, info.ItemID = taskID, itemID
	ProgressEvent.Emit(s.win, info)
}

func (s *DesktopService) CancelDownload() {
	s.downloadMu.Lock()
	defer s.downloadMu.Unlock()
	if s.cancelDownload != nil {
		s.cancelDownload()
	}
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

	imagesOnly := len(media.Images) > 0 && len(media.VideoStreams) == 0 && len(media.AudioStreams) == 0
	if (params.Mode == "images") != imagesOnly {
		return errors.New("资源类型已变化，请重新解析")
	}
	createdAt := time.Now()
	complete := func(itemID, kind, path string, resourceID int) {
		var fileSize int64
		if info, err := os.Stat(path); err == nil {
			fileSize = info.Size()
		}
		record := DownloadRecord{
			ID: params.TaskID + "-" + itemID, TaskID: params.TaskID, SiteID: media.SiteID, Kind: kind,
			Title: media.Title, SourceURL: cleanSource(params.URL), FinalPath: path, CreatedAt: createdAt,
			CompletedAt: time.Now(), Status: "done", ResourceID: resourceID, Codec: params.VideoCodec,
			FileSize: fileSize,
		}
		record.Format = strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
		if kind != "image" {
			if probe, err := downloader.ProbeMedia(ctx, path); err == nil {
				record.DurationSeconds = probe.Seconds()
			}
		}
		cover := params.CoverPreview
		if cover == "" {
			cover = params.previews[media.CoverURL]
		}
		if kind == "image" {
			for _, img := range media.Images {
				if img.Index == resourceID {
					cover = params.previews[img.URL]
					break
				}
			}
		}

		message := ""
		if params.TaskID != "" {
			var err error
			record, err = s.saveRecord(record, cover)
			if err != nil {
				message = "文件已下载，历史记录保存失败"
			}
		}
		s.emitDownload(params.TaskID, itemID, ProgressInfo{Stage: "done", FinalPath: path, Record: &record, Message: message})
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

	outputDir, err := s.ValidateDownloadDirectory(params.OutputDir)
	if err != nil {
		return err
	}

	// 创建独立临时工作目录
	workDir, err := os.MkdirTemp("", "media-fetcher-work-*")
	if err != nil {
		return fmt.Errorf("创建临时工作目录失败: %v", err)
	}
	defer os.RemoveAll(workDir)

	// LOFTER 图片下载逻辑
	if imagesOnly {
		var imagesToDownload []extractor.ImageItem
		if len(params.SelectedImage) > 0 {
			selectedMap := make(map[int]bool)
			for _, idx := range params.SelectedImage {
				selectedMap[idx] = true
			}
			for _, img := range media.Images {
				if selectedMap[img.Index] {
					imagesToDownload = append(imagesToDownload, img)
				}
			}
		} else {
			imagesToDownload = media.Images
		}

		if len(imagesToDownload) == 0 {
			return fmt.Errorf("未选择任何要下载的图片")
		}

		for i, img := range imagesToDownload {
			itemID := strconv.Itoa(img.Index)
			if err := ctx.Err(); err != nil {
				return err
			}
			filename := fmt.Sprintf("%s_%s", media.ID, img.Filename)
			if img.Filename == "" {
				filename = fmt.Sprintf("%s_%03d.jpg", media.ID, img.Index)
			}
			opts := downloader.DownloadOptions{
				OutputDir: workDir,
				Filename:  filename,
				OnProgress: func(downloaded, total int64) {
					s.emitDownload(params.TaskID, itemID, ProgressInfo{
						Stage:      "images",
						Downloaded: downloaded,
						Total:      total,
						Message:    fmt.Sprintf("下载进度: %d/%d", i+1, len(imagesToDownload)),
					})
				},
			}

			path, dlErr := d.Download(ctx, nil, img.URL, opts)
			if dlErr != nil && len(img.BackupURLs) > 0 {
				for _, backupURL := range img.BackupURLs {
					path, dlErr = d.Download(ctx, nil, backupURL, opts)
					if dlErr == nil {
						break
					}
				}
			}
			if dlErr == nil {
				path, dlErr = publishUnique(ctx, path, filepath.Join(outputDir, filepath.Base(path)))
			}
			if dlErr != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				message := "图片下载失败"
				var outputErr *outputFailure
				if errors.As(dlErr, &outputErr) {
					message = outputErr.Error()
				}
				s.emitDownload(params.TaskID, itemID, ProgressInfo{Stage: "item-error", Message: message})
				continue
			}
			complete(itemID, "image", path, img.Index)
		}

		return nil
	}

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
				s.emitDownload(params.TaskID, "media", ProgressInfo{
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
				s.emitDownload(params.TaskID, "media", ProgressInfo{
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

	format, kind, resourceID := "mp4", "video", params.VideoID
	staged := filepath.Join(workDir, "output.mp4")
	if downloadedVideo != "" && downloadedAudio != "" {
		s.emitDownload(params.TaskID, "media", ProgressInfo{Stage: "muxing"})
		if err := downloader.MuxVideoAudio(ctx, "", downloadedVideo, downloadedAudio, staged); err != nil {
			return err
		}
	} else {
		input := downloadedVideo
		if input == "" {
			input = downloadedAudio
			format, kind, resourceID = "m4a", "audio", params.AudioID
			if params.AudioFormat == "mp3" {
				format = "mp3"
			}
		}
		if input == "" {
			return errors.New("未下载任何内容")
		}
		staged, err = downloader.PrepareSingleTrack(ctx, input, filepath.Join(workDir, "output."+format), format, func(stage string) {
			s.emitDownload(params.TaskID, "media", ProgressInfo{Stage: stage})
		})
		if err != nil {
			return &outputFailure{err}
		}
	}
	finalOut, err := publishUnique(ctx, staged, filepath.Join(outputDir, title+"."+format))
	if err != nil {
		return fmt.Errorf("文件发布失败: %w", err)
	}
	complete("media", kind, finalOut, resourceID)

	return nil
}

type outputFailure struct{ error }
