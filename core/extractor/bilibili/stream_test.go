package bilibili

import (
	"encoding/json"
	"media-fetcher/core/extractor"
	"testing"
)

func TestAdvancedDASH(t *testing.T) {
	var dash rawDash
	if err := json.Unmarshal([]byte(`{"video":[{"id":125,"codecs":"hev1","base_url":"https://cdn.example/v","frame_rate":"60000/1001","mime_type":"video/mp4"},{"id":125,"codecs":"av01","baseUrl":"https://cdn.example/a","frameRate":59.94},{"id":126,"codecs":"hev1","baseUrl":"https://cdn.example/d","frameRate":"60"},{"id":127,"codecs":"av01","baseUrl":"https://cdn.example/8k"}],"audio":[{"id":30280,"codecs":"mp4a","baseUrl":"https://cdn.example/a"}],"dolby":{"audio":[{"id":30250,"codecs":"ec-3","base_url":"https://cdn.example/d","backup_url":["https://backup.example/d"]}]},"flac":{"audio":{"id":30251,"codecs":"fLaC","baseUrl":"https://cdn.example/f"}}}`), &dash); err != nil {
		t.Fatal(err)
	}
	dash.Audio = append(dash.Audio, dash.Audio[0], rawStream{ID: 1, BaseURL: "javascript:bad"})
	m := &extractor.MediaInfo{}
	appendDASH(m, dash, []supportFormat{{Quality: 125, Description: "平台 HDR"}, {Quality: 120, Description: "不得伪造4K"}})
	if len(m.VideoStreams) != 4 || len(m.AudioStreams) != 3 {
		t.Fatalf("轨道数量 %d %d", len(m.VideoStreams), len(m.AudioStreams))
	}
	if m.VideoStreams[0].FrameRate != "60000/1001" || m.VideoStreams[1].FrameRate != "59.94" || m.VideoStreams[0].DynamicRange != "HDR10" || m.VideoStreams[0].QualityLabel != "平台 HDR" {
		t.Fatal("高级字段丢失")
	}
	if m.AudioStreams[1].Kind != "dolby" || m.AudioStreams[2].Kind != "hires" || len(m.AudioStreams[1].BackupURLs) != 1 {
		t.Fatal("高级音轨丢失")
	}
	for _, body := range []string{`{}`, `{"dolby":{"audio":null},"flac":{"audio":null}}`} {
		if json.Unmarshal([]byte(body), &dash) != nil {
			t.Fatal(body)
		}
	}
	if webMediaCapabilities != 4048 {
		t.Fatal(webMediaCapabilities)
	}
}

func TestFrameRateRejectsNonFinite(t *testing.T) {
	for _, value := range []string{`"NaN"`, `"Inf"`, `"60/Inf"`, `"0"`, `"60/0"`} {
		if streamFPS(rawStream{FPS: json.RawMessage(value)}) != "" {
			t.Fatal(value)
		}
	}
}
