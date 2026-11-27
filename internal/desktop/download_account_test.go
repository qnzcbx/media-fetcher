package desktop

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"media-fetcher/core/auth"
	"media-fetcher/core/extractor"
	"media-fetcher/pkg/vault"
)

type unavailableDownloadAccount struct{ trackFixture }

func (f unavailableDownloadAccount) Extract(ctx context.Context, raw string, opt extractor.ExtractOptions) (*extractor.MediaInfo, error) {
	m, err := f.trackFixture.Extract(ctx, raw, opt)
	m.UsedAccountID = "unavailable"
	return m, err
}

func TestDownloadDoesNotFallBackToAnonymous(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "unexpected anonymous download", 403)
	}))
	defer server.Close()
	registry := extractor.NewRegistry()
	registry.Register(unavailableDownloadAccount{trackFixture{server.URL}})
	cred, err := auth.NewCredentialService(t.TempDir(), vault.NewMockVault("download-account"))
	if err != nil {
		t.Fatal(err)
	}
	s := &DesktopService{registry: registry, credService: cred, historyDir: t.TempDir()}
	err = s.doDownload(t.Context(), DownloadParams{URL: "https://www.bilibili.com/video/BV1xx411c7mD", AccountID: "unavailable", OutputDir: t.TempDir(), Mode: "complete", VideoID: 80, VideoCodec: "avc1", AudioID: 30280})
	if !errors.Is(err, extractor.ErrCredentialExpired) || requests != 0 {
		t.Fatalf("必须拒绝失效账号且不发起匿名下载: %v, 请求数 %d", err, requests)
	}
}
