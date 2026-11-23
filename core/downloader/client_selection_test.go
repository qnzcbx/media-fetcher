package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// customRoundTripper captures whether it was invoked.
type customRoundTripper struct {
	invoked bool
	base    http.RoundTripper
}

func (rt *customRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.invoked = true
	if rt.base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return rt.base.RoundTrip(req)
}

func TestDownloader_ClientSelection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	defaultTransport := &customRoundTripper{}
	defaultClient := &http.Client{
		Transport: defaultTransport,
		Timeout:   10 * time.Second,
	}

	d := New(defaultClient)

	outDir := t.TempDir()

	// 1. Download with explicit nil client should use Downloader's defaultClient.
	opts := DownloadOptions{
		OutputDir: outDir,
		Filename:  "test_nil_client.bin",
	}
	_, err := d.Download(context.Background(), nil, server.URL, opts)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if !defaultTransport.invoked {
		t.Error("Downloader did not use its defaultClient when explicit client was nil")
	}

	// Reset for next case
	defaultTransport.invoked = false

	// 2. Download with an explicit custom client should NOT use Downloader's defaultClient.
	explicitTransport := &customRoundTripper{}
	explicitClient := &http.Client{
		Transport: explicitTransport,
		Timeout:   10 * time.Second,
	}

	opts2 := DownloadOptions{
		OutputDir: outDir,
		Filename:  "test_explicit_client.bin",
	}
	_, err = d.Download(context.Background(), explicitClient, server.URL, opts2)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if defaultTransport.invoked {
		t.Error("Downloader incorrectly used its defaultClient even though an explicit client was provided")
	}
	if !explicitTransport.invoked {
		t.Error("Downloader did not use the explicitly provided client")
	}
}
