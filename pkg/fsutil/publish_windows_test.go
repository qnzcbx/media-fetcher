package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestReplacePreservesOpenWindowsDestination(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "final")
	src := filepath.Join(dir, "temp")
	for name, value := range map[string]string{dst: "old", src: "new"} {
		if err := os.WriteFile(name, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	name, err := windows.UTF16PtrFromString(dst)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	if err := PublishFile(src, dst, StrategyReplace); err == nil {
		t.Fatal("replacement unexpectedly succeeded")
	}
	for name, value := range map[string]string{dst: "old", src: "new"} {
		data, err := os.ReadFile(name)
		if err != nil || string(data) != value {
			t.Fatalf("file %s: %q, %v", name, data, err)
		}
	}
}
