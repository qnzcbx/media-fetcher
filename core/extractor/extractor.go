package extractor

import (
	"context"
	"fmt"
	"sync"
)

// Extractor is the site-adapter contract implemented by platform plugins.
type Extractor interface {
	// SiteID returns unique platform identifier (e.g. "bilibili", "lofter").
	SiteID() string
	// Match reports whether this extractor handles the given URL.
	Match(rawURL string) bool
	// Extract retrieves media information and stream descriptors from the site.
	Extract(ctx context.Context, rawURL string, opt ExtractOptions) (*MediaInfo, error)
}

// Registry manages available extractors and routes incoming URLs.
type Registry struct {
	mu         sync.RWMutex
	extractors []Extractor
}

// NewRegistry creates an empty extractor registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds an extractor to the registry.
func (r *Registry) Register(e Extractor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extractors = append(r.extractors, e)
}

// Find locates the matching extractor for a URL.
func (r *Registry) Find(rawURL string) (Extractor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, e := range r.extractors {
		if e.Match(rawURL) {
			return e, nil
		}
	}
	return nil, fmt.Errorf("%w: no extractor matched url %q", ErrInvalidURL, rawURL)
}

// All returns all registered extractors.
func (r *Registry) All() []Extractor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Extractor, len(r.extractors))
	copy(out, r.extractors)
	return out
}
