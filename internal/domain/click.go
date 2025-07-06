package domain

import (
	"net"
	"time"
)

// Click представляет клик по сокращенной ссылке
type Click struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	LinkID     int64     `gorm:"column:link_id;not null;index" json:"link_id"`
	IPAddress  *net.IP   `gorm:"column:ip_address;type:inet" json:"ip_address,omitempty"`
	UserAgent  *string   `gorm:"column:user_agent;type:text" json:"user_agent,omitempty"`
	Referer    *string   `gorm:"column:referer;size:500" json:"referer,omitempty"`
	Country    *string   `gorm:"column:country;size:2" json:"country,omitempty"` // ISO код страны
	City       *string   `gorm:"column:city;size:100" json:"city,omitempty"`
	DeviceType *string   `gorm:"column:device_type;size:10" json:"device_type,omitempty"` // 'desktop', 'mobile', 'tablet'
	Browser    *string   `gorm:"column:browser;size:50" json:"browser,omitempty"`
	OS         *string   `gorm:"column:os;size:50" json:"os,omitempty"`
	ClickedAt  time.Time `gorm:"column:clicked_at;autoCreateTime;index" json:"clicked_at"`
	IsUnique   bool      `gorm:"column:is_unique;not null;default:true" json:"is_unique"` // уникальный клик от IP за день

	// Relationships
	Link *Link `gorm:"foreignKey:LinkID" json:"link,omitempty"`
}

// TableName возвращает название таблицы для GORM
func (Click) TableName() string {
	return "clicks"
}

// GetDeviceType возвращает тип устройства для обратной совместимости
func (c *Click) GetDeviceType() string {
	if c.DeviceType != nil {
		return *c.DeviceType
	}
	return "unknown"
}