package desktop

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"media-fetcher/core/auth"
	"media-fetcher/core/extractor"
	"media-fetcher/pkg/vault"
)

type galleryFixture struct{ images []extractor.ImageItem }

func (galleryFixture) SiteID() string    { return "lofter" }
func (galleryFixture) Match(string) bool { return true }
func (f galleryFixture) Extract(context.Context, string, extractor.ExtractOptions) (*extractor.MediaInfo, error) {
	return &extractor.MediaInfo{ID: "post", SiteID: "lofter", Title: "测试", Images: f.images}, nil
}

func TestGalleryPartialDownloadAndRestart(t *testing.T) {
	var fixture bytes.Buffer
	if err := png.Encode(&fixture, image.NewRGBA(image.Rect(0, 0, 500, 250))); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/good" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(fixture.Bytes())
	}))
	defer server.Close()
	cred, err := auth.NewCredentialService(t.TempDir(), vault.NewMockVault("gallery"))
	if err != nil {
		t.Fatal(err)
	}
	registry := extractor.NewRegistry()
	registry.Register(galleryFixture{[]extractor.ImageItem{
		{Index: 1, URL: server.URL + "/good", Filename: "1.png"},
		{Index: 2, URL: server.URL + "/bad", Filename: "2.png"},
		{Index: 3, URL: server.URL + "/good", Filename: "3.png"},
	}})
	dir, out := t.TempDir(), t.TempDir()
	service := &DesktopService{registry: registry, credService: cred, historyDir: dir}
	params := DownloadParams{
		TaskID:    "0123456789abcdef0123456789abcdef",
		URL:       "https://example.lofter.com/post/abc_def?private=fixture",
		OutputDir: out, SelectedImage: []int{1, 2}, VideoID: -1, AudioID: -1,
	}
	if err := service.doDownload(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	restarted := &DesktopService{historyDir: dir}
	records, err := restarted.DownloadHistory()
	if err != nil || len(records) != 1 {
		t.Fatalf("history: %+v, %v", records, err)
	}
	record := records[0]
	if record.ResourceID != 1 || record.Status != "done" || record.SourceURL != "https://example.lofter.com/post/abc_def" || record.Thumbnail == "" {
		t.Fatalf("unexpected record %+v", record)
	}
	if data, err := restarted.GalleryThumbnail(record.ID); err != nil || data == "" {
		t.Fatalf("thumbnail unavailable: %v", err)
	}
	if err := restarted.RemoveDownloadRecord(record.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(record.FinalPath); err != nil {
		t.Fatal("original media must stay", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "thumbnails", record.Thumbnail)); !os.IsNotExist(err) {
		t.Fatal("unused thumbnail retained", err)
	}
	if records, err := restarted.DownloadHistory(); err != nil || len(records) != 0 {
		t.Fatalf("removed history: %+v, %v", records, err)
	}
	if files, err := os.ReadDir(out); err != nil || len(files) != 1 {
		t.Fatalf("unexpected output files: %+v, %v", files, err)
	}
}

func TestGallerySourceKeepsOnlyPart(t *testing.T) {
	if got := cleanSource("https://www.bilibili.com/video/BV1xx411c7mD?p=2&token=fixture#noise"); got != "https://www.bilibili.com/video/BV1xx411c7mD?p=2" {
		t.Fatal(got)
	}
}
