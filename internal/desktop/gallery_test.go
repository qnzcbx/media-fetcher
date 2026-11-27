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
		Mode:      "images",
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
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "bilibili video keeps part",
			in:   "https://www.bilibili.com/video/BV1xx411c7mD?p=2&token=fixture#noise",
			want: "https://www.bilibili.com/video/BV1xx411c7mD?p=2",
		},
		{
			name: "bilibili bangumi strips noise",
			in:   "https://www.bilibili.com/bangumi/play/ep748435/?from_spmid=abc&token=1",
			want: "https://www.bilibili.com/bangumi/play/ep748435/",
		},
		{
			name: "youtube watch keeps v only",
			in:   "https://www.youtube.com/watch?v=aqz-KE-bpKQ&list=PL123&si=tracking&t=30#frag",
			want: "https://www.youtube.com/watch?v=aqz-KE-bpKQ",
		},
		{
			name: "youtube watch drops invalid v",
			in:   "https://www.youtube.com/watch?v=invalid!id&si=abc",
			want: "https://www.youtube.com/watch",
		},
		{
			name: "youtu.be strips tracking query",
			in:   "https://youtu.be/aqz-KE-bpKQ?si=secret&t=12",
			want: "https://youtu.be/aqz-KE-bpKQ",
		},
		{
			name: "youtube shorts strips tracking query",
			in:   "https://www.youtube.com/shorts/aqz-KE-bpKQ?feature=share&si=xyz",
			want: "https://www.youtube.com/shorts/aqz-KE-bpKQ",
		},
		{
			name: "strips userinfo credentials",
			in:   "https://user:pass@www.youtube.com/watch?v=aqz-KE-bpKQ",
			want: "https://www.youtube.com/watch?v=aqz-KE-bpKQ",
		},
	}
	for _, tc := range cases {
		if got := cleanSource(tc.in); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
