//go:build !windows

package auth

import (
	"net/http"
	"net/url"
)

func systemProxyForRequest(_ *http.Request) (*url.URL, error) {
	return nil, nil
}
