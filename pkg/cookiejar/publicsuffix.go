package cookiejar

import (
	"strings"

	"golang.org/x/net/publicsuffix"
)

// IsPublicSuffix 判断给定的域名是否属于顶级域或受限公共后缀。
// 使用 golang.org/x/net/publicsuffix 保持官方且持续维护的后缀列表。
func IsPublicSuffix(domain string) bool {
	domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if domain == "" {
		return false
	}
	ps, _ := publicsuffix.PublicSuffix(domain)
	return ps == domain
}
