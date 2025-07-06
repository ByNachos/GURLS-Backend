package domain

import (
	"net"
	"time"
)

// RefreshToken представляет JWT refresh токен для веб-авторизации
type RefreshToken struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID    int64     `gorm:"column:user_id;not null;index" json:"user_id"`
	Token     string    `gorm:"column:token;size:255;uniqueIndex;not null" json:"token"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null;index" json:"expires_at"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	IsRevoked bool      `gorm:"column:is_revoked;not null;default:false" json:"is_revoked"`
	UserAgent *string   `gorm:"column:user_agent;type:text" json:"user_agent,omitempty"`
	IPAddress *net.IP   `gorm:"column:ip_address;type:inet" json:"ip_address,omitempty"`
	LastUsedAt *time.Time `gorm:"column:last_used_at" json:"last_used_at,omitempty"`

	// Relationships
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName возвращает название таблицы для GORM
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

// IsExpired проверяет, истек ли токен
func (rt *RefreshToken) IsExpired() bool {
	return time.Now().After(rt.ExpiresAt)
}

// IsValid проверяет, является ли токен валидным
func (rt *RefreshToken) IsValid() bool {
	return !rt.IsExpired() && !rt.IsRevoked && rt.Token != ""
}

// Revoke отзывает токен
func (rt *RefreshToken) Revoke() {
	rt.IsRevoked = true
}

// UpdateLastUsed обновляет время последнего использования
func (rt *RefreshToken) UpdateLastUsed() {
	now := time.Now()
	rt.LastUsedAt = &now
}