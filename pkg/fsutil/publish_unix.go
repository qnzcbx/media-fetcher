//go:build !windows
// +build !windows

package fsutil

import (
	"os"
)

func publishNoOverwrite(tempPath, finalPath string) error {
	// 简单的硬链接然后再移除临时文件，能保证目标文件不存在时才成功
	// 如果目标已存在，os.Link会失败 (os.IsExist)
	err := os.Link(tempPath, finalPath)
	if err != nil {
		if os.IsExist(err) {
			return ErrFileExists
		}
		return err
	}
	return os.Remove(tempPath)
}

func publishReplace(tempPath, finalPath string) error {
	// Unix 平台上的 rename(2) 会自动且原子地覆盖已存在的目标文件
	return os.Rename(tempPath, finalPath)
}
