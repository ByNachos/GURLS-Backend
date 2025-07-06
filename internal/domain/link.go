package domain

import "time"

type Link struct {
	ID              int64      `gorm:"primaryKey;column:id" json:"id"`
	UserID          int64      `gorm:"column:user_id;not null;index" json:"user_id"`
	OriginalURL     string     `gorm:"column:original_url;type:text;not null" json:"original_url"`
	Alias           string     `gorm:"column:alias;size:20;uniqueIndex;not null" json:"alias"`
	Title           *string    `gorm:"column:title;size:200" json:"title,omitempty"`
	Description     *string    `gorm:"column:description;size:500" json:"description,omitempty"`
	ExpiresAt       *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
	MaxClicks       *int       `gorm:"column:max_clicks" json:"max_clicks,omitempty"`
	ClickCount      int64      `gorm:"column:click_count;default:0" json:"click_count"`
	PasswordHash    *string    `gorm:"column:password_hash;size:60" json:"-"` // скрываем пароль в JSON
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	IsActive        bool       `gorm:"column:is_active;default:true" json:"is_active"`

	// Relationships
	User   *User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Clicks []Click `gorm:"foreignKey:LinkID" json:"clicks,omitempty"`

	// Backward compatibility - это поле больше не сохраняется в БД,
	// но может вычисляться динамически для совместимости
	ClicksByDevice map[string]int64 `gorm:"-" json:"clicks_by_device,omitempty"`
}

// TableName возвращает название таблицы для GORM
func (Link) TableName() string {
	return "links"
}
