package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishFile_StrategyCreateNew(t *testing.T) {
	tmpDir := t.TempDir()
	tempPath := filepath.Join(tmpDir, "temp.txt")
	finalPath := filepath.Join(tmpDir, "final.txt")

	if err := os.WriteFile(tempPath, []byte("temp"), 0644); err != nil {
		t.Fatal(err)
	}

	err := PublishFile(tempPath, finalPath, StrategyCreateNew)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should not be overwritten
	if err := os.WriteFile(tempPath, []byte("temp2"), 0644); err != nil {
		t.Fatal(err)
	}
	err = PublishFile(tempPath, finalPath, StrategyCreateNew)
	if err != ErrFileExists {
		t.Fatalf("expected ErrFileExists, got: %v", err)
	}

	content, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "temp" {
		t.Fatalf("expected temp, got %s", string(content))
	}
}

func TestPublishFile_StrategyReplace(t *testing.T) {
	tmpDir := t.TempDir()
	tempPath := filepath.Join(tmpDir, "temp.txt")
	finalPath := filepath.Join(tmpDir, "final.txt")

	if err := os.WriteFile(finalPath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tempPath, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	err := PublishFile(tempPath, finalPath, StrategyReplace)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Fatalf("expected new, got %s", string(content))
	}
}
