package lofter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"media-fetcher/core/extractor"
)

func TestLofterExtractor_Match(t *testing.T) {
	ext := New(nil)

	valid := []string{
		"https://duzou85951.lofter.com/post/4d0c4f36_34cd81d98",
		"http://user.lofter.com/post/12ab_34cd?incantation=abc",
	}
	for _, u := range valid {
		if !ext.Match(u) {
			t.Errorf("expected match for %s", u)
		}
	}

	invalid := []string{
		"https://lofter.com/",
		"https://bilibili.com/video/BV123",
		"https://other.com/post/123",
	}
	for _, u := range invalid {
		if ext.Match(u) {
			t.Errorf("expected no match for %s", u)
		}
	}
}

func TestLofterExtractor_ExtractPhotoPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		// targetblogid=1292652342, postid=14174133656
		if r.FormValue("targetblogid") != "1292652342" || r.FormValue("postid") != "14174133656" {
			http.Error(w, "wrong form parameters", 400)
			return
		}

		payload := `{
			"meta": {"status": 200, "msg": "ok"},
			"response": {
				"posts": [{
					"post": {
						"type": 2,
						"title": "测试图片帖",
						"blogInfo": {"blogNickName": "画师小明"},
						"photoLinks": "[{\"orign\":\"https://img.example.org/original_1.jpg?token=abc\"},{\"orign\":\"https://img.example.org/original_2.jpg\"}]"
					}
				}]
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	ext := New(nil)
	ext.SetAPIURL(server.URL)

	testURL := "https://duzou85951.lofter.com/post/4d0c4f36_34cd81d98"
	media, err := ext.Extract(context.Background(), testURL, extractor.ExtractOptions{})
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if media.SiteID != "lofter" || media.ID != "14174133656" {
		t.Errorf("media metadata mismatch: %+v", media)
	}
	if media.Author != "画师小明" || media.Title != "测试图片帖" {
		t.Errorf("title/author mismatch: %s / %s", media.Title, media.Author)
	}
	if len(media.Images) != 2 {
		t.Fatalf("expected 2 images, got: %d", len(media.Images))
	}
	if media.Images[0].URL != "https://img.example.org/original_1.jpg?token=abc" {
		t.Errorf("CDN signature stripped or url mismatch: %s", media.Images[0].URL)
	}
}

func TestLofterExtractor_ExtractArticlePost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := `{
			"meta": {"status": 200, "msg": "ok"},
			"response": {
				"posts": [{
					"post": {
						"type": 1,
						"digest": "文章摘要",
						"blogInfo": {"blogNickName": "博主"},
						"content": "<p>文章段落</p><img src=\"https://img.example.com/art1.png\"><img data-src=\"https://img.example.com/art2.png\">"
					}
				}]
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	ext := New(nil)
	ext.SetAPIURL(server.URL)

	testURL := "https://duzou85951.lofter.com/post/4d0c4f36_34cd81d98"
	media, err := ext.Extract(context.Background(), testURL, extractor.ExtractOptions{})
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	if len(media.Images) != 2 {
		t.Fatalf("expected 2 article images, got %d", len(media.Images))
	}
}
