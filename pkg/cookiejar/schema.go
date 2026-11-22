package cookiejar

import "time"

// CurrentSchemaVersion denotes the format version of serialized credentials.
const CurrentSchemaVersion = 1

// StorageSchema defines the versioned JSON structure for serialized CookieJar.
type StorageSchema struct {
	Version   int           `json:"version"`
	SiteID    string        `json:"site_id"`
	AccountID string        `json:"account_id"`
	UpdatedAt time.Time     `json:"updated_at"`
	Cookies   []CookieEntry `json:"cookies"`
}
