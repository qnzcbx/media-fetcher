package desktop

import (
	"context"
	"time"

	coreruntime "media-fetcher/core/runtime"
)

// RuntimeComponentStatus 描述设置页展示的单项运行组件状态。
type RuntimeComponentStatus struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Version   string `json:"version"`
	Source    string `json:"source"`
	Kind      string `json:"kind"`
	Detail    string `json:"detail"`
	Path      string `json:"path"`
	PairPath  string `json:"pairPath"`
}

// RuntimeStatusResponse 描述桌面端三类运行组件的实时状态。
type RuntimeStatusResponse struct {
	YTDLP     RuntimeComponentStatus `json:"ytdlp"`
	JSRuntime RuntimeComponentStatus `json:"jsRuntime"`
	FFmpeg    RuntimeComponentStatus `json:"ffmpeg"`
}

func mapComponentStatus(st coreruntime.ComponentStatus) RuntimeComponentStatus {
	return RuntimeComponentStatus{
		Key:       st.Key,
		Name:      st.Name,
		Available: st.Available,
		Version:   st.Version,
		Source:    string(st.Source),
		Kind:      st.Kind,
		Detail:    st.Detail,
		Path:      st.Path,
		PairPath:  st.PairPath,
	}
}

// GetRuntimeStatus 返回下载引擎（yt-dlp）、JS 运行时与媒体处理（FFmpeg / ffprobe）的可用状态与来源。
func (s *DesktopService) GetRuntimeStatus() RuntimeStatusResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	snap := coreruntime.InspectStatus(ctx, coreruntime.Options{})
	return RuntimeStatusResponse{
		YTDLP:     mapComponentStatus(snap.YTDLP),
		JSRuntime: mapComponentStatus(snap.JSRuntime),
		FFmpeg:    mapComponentStatus(snap.FFmpeg),
	}
}
