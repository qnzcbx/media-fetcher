//go:build windows

package desktop

import "golang.org/x/sys/windows"

func systemDownloadDirectory()(string,error) {
 return windows.KnownFolderPath(windows.FOLDERID_Downloads,0)
}
