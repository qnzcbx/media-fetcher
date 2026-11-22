package auth

import "time"

// AccountStatus 表示账号的认证生命周期状态。
type AccountStatus string

const (
	StatusActive    AccountStatus = "active"
	StatusExpired   AccountStatus = "expired"
	StatusLoggedOut AccountStatus = "logged_out"
)

// Account 表示特定站点上的已认证用户账号。
// 敏感数据（Cookie、Token 等）由 PersistentCookieJar 和 Vault 统一安全加密管理，
// 严禁在此结构体中直接存储明文凭证，也绝不会在 JSON 序列化中暴露。
type Account struct {
	SiteID          string            `json:"site_id"`
	AccountID       string            `json:"account_id"`
	Name            string            `json:"name"`
	AvatarURL       string            `json:"avatar_url,omitempty"`
	Status          AccountStatus     `json:"status"`
	IsActive        bool              `json:"is_active"`
	CreatedAt       time.Time         `json:"created_at"`
	LastValidatedAt time.Time         `json:"last_validated_at"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// SafeAccount 是提供给前端/UI 和日志展示的安全脱敏账号实体。
type SafeAccount struct {
	SiteID          string        `json:"site_id"`
	AccountID       string        `json:"account_id"`
	Name            string        `json:"name"`
	AvatarURL       string        `json:"avatar_url,omitempty"`
	Status          AccountStatus `json:"status"`
	IsActive        bool          `json:"is_active"`
	LastValidatedAt time.Time     `json:"last_validated_at"`
}

// ToSafe 返回适用于 UI 渲染与日志输出的安全脱敏 SafeAccount。
func (a *Account) ToSafe() SafeAccount {
	return SafeAccount{
		SiteID:          a.SiteID,
		AccountID:       a.AccountID,
		Name:            a.Name,
		AvatarURL:       a.AvatarURL,
		Status:          a.Status,
		IsActive:        a.IsActive,
		LastValidatedAt: a.LastValidatedAt,
	}
}
