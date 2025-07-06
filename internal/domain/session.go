package domain

import (
	"net"
	"time"
)

// Session представляет пользовательскую сессию для веб-авторизации
type Session struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID       int64     `gorm:"column:user_id;not null;index" json:"user_id"`
	SessionToken string    `gorm:"column:session_token;size:32;uniqueIndex;not null" json:"session_token"`
	ExpiresAt    time.Time `gorm:"column:expires_at;not null;index" json:"expires_at"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UserAgent    *string   `gorm:"column:user_agent;type:text" json:"user_agent,omitempty"`
	IPAddress    *net.IP   `gorm:"column:ip_address;type:inet" json:"ip_address,omitempty"`

	// Relationships
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName возвращает название таблицы для GORM
func (Session) TableName() string {
	return "sessions"
}

// IsExpired проверяет, истекла ли сессия
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsValid проверяет, является ли сессия валидной
func (s *Session) IsValid() bool {
	return !s.IsExpired() && s.SessionToken != ""
}

// ExtendExpiration продлевает срок действия сессии
func (s *Session) ExtendExpiration(duration time.Duration) {
	s.ExpiresAt = time.Now().Add(duration)
}