package cookiejar

import (
	"strings"
)

// publicSuffixes 包含了常见的顶级域名与二级公共后缀列表。
// 依据 RFC 6265 5.3 节规则 5，用户代理必须拒绝将 Cookie 的 Domain 设置为公共后缀，
// 防止恶意或不规范响应将 Cookie 范围扩大至整个顶级域或公共二级域（如 .com 或 .com.cn）。
var publicSuffixes = map[string]bool{
	// 常见 gTLD
	"com": true, "net": true, "org": true, "edu": true, "gov": true, "mil": true,
	"info": true, "biz": true, "name": true, "pro": true, "xyz": true, "top": true,
	"tech": true, "site": true, "online": true, "vip": true, "club": true, "app": true,
	"dev": true, "io": true, "ai": true, "me": true, "tv": true, "cc": true, "co": true,

	// 常见 ccTLD
	"cn": true, "us": true, "uk": true, "jp": true, "kr": true, "de": true, "fr": true,
	"ru": true, "hk": true, "tw": true, "mo": true, "sg": true, "ca": true, "au": true,

	// 常见二级公共后缀
	"com.cn": true, "net.cn": true, "org.cn": true, "gov.cn": true, "edu.cn": true,
	"ac.cn": true, "ah.cn": true, "bj.cn": true, "cq.cn": true, "fj.cn": true,
	"gd.cn": true, "gs.cn": true, "gz.cn": true, "gx.cn": true, "ha.cn": true,
	"hb.cn": true, "he.cn": true, "hi.cn": true, "hl.cn": true, "hn.cn": true,
	"jl.cn": true, "js.cn": true, "jx.cn": true, "ln.cn": true, "nm.cn": true,
	"nx.cn": true, "qh.cn": true, "sc.cn": true, "sd.cn": true, "sh.cn": true,
	"sn.cn": true, "sx.cn": true, "tj.cn": true, "xj.cn": true, "xz.cn": true,
	"yn.cn": true, "zj.cn": true,

	"co.uk": true, "org.uk": true, "me.uk": true, "ltd.uk": true, "plc.uk": true,
	"co.jp": true, "ne.jp": true, "or.jp": true, "ac.jp": true, "go.jp": true,
	"com.hk": true, "org.hk": true, "net.hk": true, "edu.hk": true, "gov.hk": true,
	"com.tw": true, "org.tw": true, "net.tw": true, "edu.tw": true, "gov.tw": true,
	"com.sg": true, "org.sg": true, "net.sg": true, "edu.sg": true, "gov.sg": true,
	"co.kr": true, "ne.kr": true, "or.kr": true, "re.kr": true,
}

// IsPublicSuffix 判断给定的域名是否属于顶级域或受限公共后缀。
func IsPublicSuffix(domain string) bool {
	domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	return publicSuffixes[domain]
}
