//go:build !windows

package desktop

import (
	"errors"
	"fmt"
	"os"
)

func fileIdentity(info os.FileInfo) string {
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
}
func safeLocalPath(string) error { return errors.New("此平台暂不支持文件回收") }
func recycleFile(string) error   { return errors.New("此平台暂不支持文件回收") }
