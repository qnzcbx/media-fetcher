package desktop

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	_ "golang.org/x/image/webp"
	"media-fetcher/core/downloader"
	"media-fetcher/pkg/fsutil"
)

type DownloadRecord struct {
	DurationSeconds float64   `json:"durationSeconds,omitempty"`
	Format          string    `json:"format,omitempty"`
	ID              string    `json:"id"`
	TaskID          string    `json:"taskId"`
	SiteID          string    `json:"siteId"`
	Kind            string    `json:"kind"`
	Title           string    `json:"title"`
	SourceURL       string    `json:"sourceUrl"`
	FinalPath       string    `json:"finalPath"`
	Thumbnail       string    `json:"thumbnail"`
	CreatedAt       time.Time `json:"createdAt"`
	CompletedAt     time.Time `json:"completedAt"`
	Status          string    `json:"status"`
	ResourceID      int       `json:"resourceId"`
	Codec           string    `json:"codec"`
	FileSize        int64     `json:"fileSize,omitempty"`
}

var taskIDPattern = regexp.MustCompile("^[a-f0-9]{32}$")

func historyDirectory() (string, error) {
	root := os.Getenv("APPDATA")
	if root != "" {
		return filepath.Join(root, "media-fetcher", "downloads"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".media-fetcher", "downloads"), nil
}

func readHistory(dir string) ([]DownloadRecord, error) {
	data, err := os.ReadFile(filepath.Join(dir, "history.json"))
	if errors.Is(err, os.ErrNotExist) {
		return []DownloadRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	records := []DownloadRecord{}
	if err = json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	for i := range records {
		r := &records[i]
		if r.FileSize == 0 && r.FinalPath != "" {
			if info, err := os.Stat(r.FinalPath); err == nil && info.Mode().IsRegular() {
				r.FileSize = info.Size()
			}
		}
		if r.Format == "" {
			r.Format = strings.TrimPrefix(strings.ToLower(filepath.Ext(r.FinalPath)), ".")
		}
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].CreatedAt.After(records[j].CreatedAt) })
	return records, nil
}

func publishHistoryFile(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".gallery-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if err = errors.Join(err, f.Close()); err != nil {
		return err
	}
	return fsutil.PublishFile(f.Name(), filepath.Join(dir, name), fsutil.StrategyReplace)
}

func writeHistory(dir string, records []DownloadRecord) error {
	data, err := json.Marshal(records)
	if err != nil {
		return err
	}
	return publishHistoryFile(dir, "history.json", data)
}

func cleanSource(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	page := u.Query().Get("p")
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.User = nil
	if n, err := strconv.Atoi(page); err == nil && n > 0 {
		u.RawQuery = "p=" + strconv.Itoa(n)
	}
	return u.String()
}

func makeThumbnail(data []byte) ([]byte, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 32_000_000 {
		return nil, errors.New("无法识别缩略图或尺寸过大")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	width, height := config.Width, config.Height
	scale := float64(360) / float64(max(width, height))
	if scale < 1 {
		width = max(1, int(float64(width)*scale))
		height = max(1, int(float64(height)*scale))
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dst.Set(x, y, src.At(src.Bounds().Min.X+x*config.Width/width, src.Bounds().Min.Y+y*config.Height/height))
		}
	}
	var out bytes.Buffer
	err = jpeg.Encode(&out, dst, &jpeg.Options{Quality: 82})
	return out.Bytes(), err
}

func thumbnailInput(record DownloadRecord, cover string) ([]byte, error) {
	if record.Kind != "image" {
		_, encoded, ok := strings.Cut(cover, ",")
		if !ok || len(encoded) > base64.StdEncoding.EncodedLen(2<<20) {
			return nil, errors.New("封面不可用")
		}
		return base64.StdEncoding.DecodeString(encoded)
	}
	f, err := os.Open(record.FinalPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (24<<20)+1))
	if len(data) > 24<<20 {
		return nil, errors.New("图片超过缩略图大小限制")
	}
	return data, err
}

func (s *DesktopService) saveRecord(record DownloadRecord, cover string) (DownloadRecord, error) {
	if s.historyDir == "" {
		return record, nil
	}
	data, err := thumbnailInput(record, cover)
	if err == nil {
		data, err = makeThumbnail(data)
	}
	if err != nil && record.Kind == "image" && cover != "" {
		data, err = thumbnailInput(DownloadRecord{}, cover)
		if err == nil {
			data, err = makeThumbnail(data)
		}
	}
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	records, readErr := readHistory(s.historyDir)
	if readErr != nil {
		return record, readErr
	}
	if err == nil {
		name := record.ID + ".jpg"
		if publishHistoryFile(filepath.Join(s.historyDir, "thumbnails"), name, data) == nil {
			record.Thumbnail = name
		}
	}
	err = writeHistory(s.historyDir, append([]DownloadRecord{record}, records...))
	if err != nil && record.Thumbnail != "" {
		_ = os.Remove(filepath.Join(s.historyDir, "thumbnails", record.Thumbnail))
		record.Thumbnail = ""
	}
	return record, err
}

func (s *DesktopService) DownloadHistory() ([]DownloadRecord, error) {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	return readHistory(s.historyDir)
}

func (s *DesktopService) GalleryThumbnail(id string) (string, error) {
	records, err := s.DownloadHistory()
	if err != nil {
		return "", err
	}
	for _, r := range records {
		if r.ID == id && r.Thumbnail == r.ID+".jpg" && r.Thumbnail == filepath.Base(r.Thumbnail) {
			f, err := os.Open(filepath.Join(s.historyDir, "thumbnails", r.Thumbnail))
			if err != nil {
				return "", err
			}
			defer f.Close()
			data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
			if len(data) > 1<<20 {
				return "", errors.New("缩略图大小超出限制")
			}
			if err != nil {
				return "", err
			}
			return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data), nil
		}
	}
	return "", nil
}

func (s *DesktopService) RemoveDownloadRecord(id string) error {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	records, err := readHistory(s.historyDir)
	if err != nil {
		return err
	}
	for i, r := range records {
		if r.ID != id {
			continue
		}
		records = append(records[:i], records[i+1:]...)
		if err = writeHistory(s.historyDir, records); err != nil {
			return err
		}
		if r.Thumbnail == r.ID+".jpg" && r.Thumbnail == filepath.Base(r.Thumbnail) {
			for _, other := range records {
				if other.Thumbnail == r.Thumbnail {
					return nil
				}
			}
			_ = os.Remove(filepath.Join(s.historyDir, "thumbnails", r.Thumbnail))
		}
		return nil
	}
	return nil
}

func (s *DesktopService) OpenDownloadRecord(id, action string) error {
	records, err := s.DownloadHistory()
	if err != nil {
		return err
	}
	for _, r := range records {
		if r.ID != id {
			continue
		}
		switch action {
		case "source":
			if source := cleanSource(r.SourceURL); source != "" {
				return mygo.Shell.OpenExternal(source)
			}
		case "file", "folder":
			path := r.FinalPath
			if action == "folder" {
				path = filepath.Dir(path)
			}
			if _, err := os.Stat(path); err != nil {
				return errors.New("本地文件或目录已移动或删除")
			}
			return mygo.Shell.OpenPath(path)
		}
	}
	return fmt.Errorf("无法打开这条下载记录")
}

// DownloadDuration 按需补读旧记录，工具或文件不可用时仍保留历史。
func (s *DesktopService) DownloadDuration(id string) float64 {
	records, err := s.DownloadHistory()
	if err != nil {
		return 0
	}
	for _, record := range records {
		if record.ID != id || (record.Kind != "video" && record.Kind != "audio") {
			continue
		}
		if record.DurationSeconds > 0 {
			return record.DurationSeconds
		}
		info, err := os.Stat(record.FinalPath)
		if err != nil || !info.Mode().IsRegular() {
			return 0
		}
		probe, err := downloader.ProbeMedia(context.Background(), record.FinalPath)
		if err != nil {
			return 0
		}
		return probe.Seconds()
	}
	return 0
}
