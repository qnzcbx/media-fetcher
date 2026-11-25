package desktop

import (
	"context"
	"testing"

	"media-fetcher/core/extractor"
)

type blockedDirectoryExtractor struct{ started, finished chan struct{} }

func (blockedDirectoryExtractor) SiteID() string    { return "bilibili" }
func (blockedDirectoryExtractor) Match(string) bool { return true }
func (f blockedDirectoryExtractor) Extract(ctx context.Context, _ string, _ extractor.ExtractOptions) (*extractor.MediaInfo, error) {
	close(f.started)
	<-ctx.Done()
	close(f.finished)
	return nil, ctx.Err()
}

func TestDesktopService_StartDownload_Concurrency(t *testing.T) {
	s, err := NewDesktopService()
	if err != nil {
		t.Fatal(err)
	}
	fixture := blockedDirectoryExtractor{make(chan struct{}), make(chan struct{})}
	s.registry = extractor.NewRegistry()
	s.registry.Register(fixture)
	params := DownloadParams{OutputDir: t.TempDir(), URL: "fixture", Mode: "video", VideoID: 80, AudioID: -1}
	if err := s.StartDownload(params); err != nil {
		t.Fatal(err)
	}
	<-fixture.started
	if err := s.StartDownload(params); err == nil {
		t.Fatal("并发任务被接受")
	}
	s.CancelDownload()
	<-fixture.finished
}
