package extractor

import "errors"

var (
	// ErrNetworkFailed 表示网络连接失败、超时或 DNS 解析异常。
	ErrNetworkFailed = errors.New("extractor: 网络请求失败")
	// ErrAPIDenied 表示上游接口拒绝访问或返回业务错误码。
	ErrAPIDenied = errors.New("extractor: 接口拒绝访问或返回错误")
	// ErrCredentialExpired 表示当前凭证无效、已过期或已被注销。
	ErrCredentialExpired = errors.New("extractor: 凭证无效或已过期")
	// ErrResourceExpired 表示 CDN 流地址已过期或媒体资源已失效。
	ErrResourceExpired = errors.New("extractor: 媒体资源或流地址已过期")
	// ErrUnsupportedMedia 表示遇到不支持的媒体格式、编码或结构。
	ErrUnsupportedMedia = errors.New("extractor: 不支持的媒体类型或格式")
	// ErrInvalidURL 表示 URL 格式非法或无法识别。
	ErrInvalidURL = errors.New("extractor: 无效或不受支持的 URL")
)
