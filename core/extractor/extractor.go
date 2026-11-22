package extractor

import (
	"context"
	"fmt"
	"sync"
)

// Extractor 是各平台独立站点解析适配器必须实现的契约接口。
type Extractor interface {
	// SiteID 返回平台唯一标识（例如 "bilibili"、"lofter"）。
	SiteID() string
	// Match 判断当前解析器是否能够处理给定的 URL。
	Match(rawURL string) bool
	// Extract 从站点检索解析媒体信息与实际可用流描述符。
	Extract(ctx context.Context, rawURL string, opt ExtractOptions) (*MediaInfo, error)
}

// Registry 统一管理已注册的站点解析器，并负责为目标 URL 进行路由分发。
type Registry struct {
	mu         sync.RWMutex
	extractors []Extractor
}

// NewRegistry 创建空的解析器注册中心实例。
func NewRegistry() *Registry {
	return &Registry{}
}

// Register 向注册中心登记一个站点解析器。
func (r *Registry) Register(e Extractor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extractors = append(r.extractors, e)
}

// Find 根据传入的 URL 匹配并返回对应的站点解析器。
func (r *Registry) Find(rawURL string) (Extractor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, e := range r.extractors {
		if e.Match(rawURL) {
			return e, nil
		}
	}
	return nil, fmt.Errorf("%w: 未找到能处理 URL %q 的站点解析器", ErrInvalidURL, rawURL)
}

// All 返回当前所有已注册解析器的切片快照。
func (r *Registry) All() []Extractor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Extractor, len(r.extractors))
	copy(out, r.extractors)
	return out
}
