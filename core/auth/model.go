package auth

import "time"

// AccountStatus represents the authentication lifecycle state of an account.
type AccountStatus string

const (
	StatusActive    AccountStatus = "active"
	StatusExpired   AccountStatus = "expired"
	StatusLoggedOut AccountStatus = "logged_out"
)

// Account represents an authenticated user on a specific site.
// Sensitive data (Cookies, Tokens) are strictly managed via PersistentCookieJar and Vault,
// and are NEVER stored in this struct or exposed in JSON.
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

// SafeAccount is the sanitized representation exposed to frontend/UI and logs.
type SafeAccount struct {
	SiteID          string        `json:"site_id"`
	AccountID       string        `json:"account_id"`
	Name            string        `json:"name"`
	AvatarURL       string        `json:"avatar_url,omitempty"`
	Status          AccountStatus `json:"status"`
	IsActive        bool          `json:"is_active"`
	LastValidatedAt time.Time     `json:"last_validated_at"`
}

// ToSafe returns the sanitized SafeAccount suitable for UI display.
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
