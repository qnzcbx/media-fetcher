//go:build windows
// +build windows

package fsutil

import (
	"os"

	"golang.org/x/sys/windows"
)

func publishNoOverwrite(tempPath, finalPath string) error {
	from, err := windows.UTF16PtrFromString(tempPath)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(finalPath)
	if err != nil {
		return err
	}
	// MoveFileEx without MOVEFILE_REPLACE_EXISTING
	err = windows.MoveFileEx(from, to, 0)
	if err != nil {
		if err == windows.ERROR_ALREADY_EXISTS {
			return ErrFileExists
		}
		return &os.LinkError{Op: "rename", Old: tempPath, New: finalPath, Err: err}
	}
	return nil
}

func publishReplace(tempPath, finalPath string) error {
	from, err := windows.UTF16PtrFromString(tempPath)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(finalPath)
	if err != nil {
		return err
	}
	// On Windows, use MoveFileEx with MOVEFILE_REPLACE_EXISTING and MOVEFILE_WRITE_THROUGH (optional).
	// We use MOVEFILE_REPLACE_EXISTING.
	err = windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING)
	if err != nil {
		if err == windows.ERROR_ACCESS_DENIED {
			// This could happen if the file is in use. We should NOT delete the existing file and try again!
			// We return an error.
		}
		return &os.LinkError{Op: "rename", Old: tempPath, New: finalPath, Err: err}
	}
	return nil
}
