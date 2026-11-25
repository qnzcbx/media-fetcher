//go:build !windows

package desktop

import "os"

func systemDownloadDirectory()(string,error) {
 return fallbackDownloadDirectory(os.UserHomeDir)
}
