package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadDirectoryValidation(t *testing.T) {
	s := &DesktopService{}
	root := t.TempDir()
	valid := filepath.Join(root, "中文 空格")
	if err := os.Mkdir(valid, 0700); err != nil {
		t.Fatal(err)
	}
	if path, err := s.ValidateDownloadDirectory(valid); err != nil || path != valid {
		t.Fatalf("%s %v", path, err)
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"relative", filepath.Join(root, "missing"), file} {
		if _, err := s.ValidateDownloadDirectory(path); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	files, err := os.ReadDir(valid)
	if err != nil || len(files) != 0 {
		t.Fatalf("probe residue: %v %v", files, err)
	}
	if err := os.Remove(valid); err != nil {
		t.Fatal(err)
	}
	params := DownloadParams{Mode: "video", VideoID: 80, AudioID: -1, OutputDir: valid}
	if err := s.StartDownload(params); err == nil {
		t.Fatal("invalid directory accepted")
	}
	if _, err := os.Stat(valid); !os.IsNotExist(err) {
		t.Fatal("deleted directory recreated")
	}
}
func TestDownloadDirectoryFallback(t *testing.T) {
	if _, err := fallbackDownloadDirectory(func() (string, error) { return "", errors.New("no home") }); err == nil {
		t.Fatal("failure ignored")
	}
	if _, err := fallbackDownloadDirectory(func() (string, error) { return ".", nil }); err == nil {
		t.Fatal("relative home accepted")
	}
	root := t.TempDir()
	if path, err := fallbackDownloadDirectory(func() (string, error) { return root, nil }); err != nil || path != filepath.Join(root, "Downloads") {
		t.Fatalf("%s %v", path, err)
	}
}
