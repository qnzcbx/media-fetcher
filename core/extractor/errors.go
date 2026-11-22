package extractor

import "errors"

var (
	// ErrNetworkFailed represents network connectivity issues, timeouts, or DNS failures.
	ErrNetworkFailed = errors.New("extractor: network operation failed")
	// ErrAPIDenied represents upstream service returning non-zero error or access denial.
	ErrAPIDenied = errors.New("extractor: API request denied or returned error")
	// ErrCredentialExpired represents invalid, expired or revoked user credentials.
	ErrCredentialExpired = errors.New("extractor: credentials expired or invalid")
	// ErrResourceExpired represents expired CDN stream URL or media resource.
	ErrResourceExpired = errors.New("extractor: media resource or stream link has expired")
	// ErrUnsupportedMedia represents media formats, types or sites not supported.
	ErrUnsupportedMedia = errors.New("extractor: unsupported media type or structure")
	// ErrInvalidURL represents malformed, unparseable or unrecognized URLs.
	ErrInvalidURL = errors.New("extractor: invalid or unsupported URL format")
)
