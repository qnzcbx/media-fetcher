package cookiejar

import "time"

// CurrentSchemaVersion 表示序列化凭证数据的格式版本号。
const CurrentSchemaVersion = 1

// StorageSchema 定义序列化 CookieJar 的版本化 JSON 存储结构。
type StorageSchema struct {
	Version   int           `json:"version"`
	SiteID    string        `json:"site_id"`
	AccountID string        `json:"account_id"`
	UpdatedAt time.Time     `json:"updated_at"`
	Cookies   []CookieEntry `json:"cookies"`
}
