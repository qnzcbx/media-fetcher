package desktop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"media-fetcher/pkg/fsutil"
)

func validateDownload(p DownloadParams) error {
	hasVideo := p.VideoID >= 0 || p.VideoKey != ""
	hasAudio := p.AudioID >= 0 || p.AudioKey != ""
	switch p.Mode {
	case "images":
		if len(p.SelectedImage) > 0 && !hasAudio && !hasVideo && p.AudioFormat == "" {
			return nil
		}
	case "complete":
		if hasVideo && hasAudio && p.AudioFormat == "" && len(p.SelectedImage) == 0 {
			return nil
		}
	case "video":
		if hasVideo && !hasAudio && p.AudioFormat == "" && len(p.SelectedImage) == 0 {
			return nil
		}
	case "audio":
		if !hasVideo && hasAudio && (p.AudioFormat == "original" || p.AudioFormat == "m4a" || p.AudioFormat == "mp3") && len(p.SelectedImage) == 0 {
			return nil
		}
	}
	return errors.New("下载模式与所选资源或输出格式不匹配")
}

type publishReader struct {
	ctx context.Context
	r   io.Reader
}

func (r publishReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// publishUnique 在目标卷准备文件，再以不覆盖的方式发布。
func publishUnique(ctx context.Context, src, dst string) (string, error) {
	dir := filepath.Dir(dst)
	if _, err := (&DesktopService{}).resolveDownloadDirectory(dir); err != nil {
		return "", &outputFailure{err}
	}
	staged, err := os.CreateTemp(dir, ".publish-*")
	if err != nil {
		return "", publishDirectoryError(dir, err)
	}
	defer os.Remove(staged.Name())
	input, err := os.Open(src)
	if err != nil {
		_ = staged.Close()
		return "", err
	}
	_, err = io.Copy(staged, publishReader{ctx, input})
	_ = input.Close()
	if err == nil {
		err = staged.Sync()
	}
	if err = errors.Join(err, staged.Close()); err != nil {
		return "", err
	}
	ext := filepath.Ext(dst)
	base := strings.TrimSuffix(dst, ext)
	for i := 0; i < 10000; i++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		candidate := dst
		if i > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", base, i, ext)
		}
		err := fsutil.PublishFile(staged.Name(), candidate, fsutil.StrategyCreateNew)
		if err == nil {
			return candidate, nil
		}
		if !errors.Is(err, fsutil.ErrFileExists) && !os.IsExist(err) {
			if _, statErr := os.Lstat(candidate); statErr != nil {
				return "", publishDirectoryError(dir, err)
			}
		}
	}
	return "", errors.New("同名文件数量超出限制")
}

// publishDirectoryError 为发布期间失效的目标目录保留可读提示。
func publishDirectoryError(dir string, cause error) error {
	if _, err := (&DesktopService{}).resolveDownloadDirectory(dir); err != nil {
		return &outputFailure{err}
	}
	return cause
}
