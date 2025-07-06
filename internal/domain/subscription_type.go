package domain

import "time"

// SubscriptionType представляет тип подписки
type SubscriptionType struct {
	ID                     int16   `gorm:"primaryKey;column:id" json:"id"`
	Name                   string  `gorm:"column:name;size:20;uniqueIndex;not null" json:"name"`
	DisplayName            string  `gorm:"column:display_name;size:50;not null" json:"display_name"`
	PriceMonthly           float64 `gorm:"column:price_monthly;type:decimal(6,2);not null;default:0.00" json:"price_monthly"`
	PriceYearly            float64 `gorm:"column:price_yearly;type:decimal(7,2);not null;default:0.00" json:"price_yearly"`
	MaxLinksPerMonth       *int    `gorm:"column:max_links_per_month" json:"max_links_per_month,omitempty"` // NULL = unlimited
	MaxClicksPerMonth      *int    `gorm:"column:max_clicks_per_month" json:"max_clicks_per_month,omitempty"` // NULL = unlimited
	AnalyticsRetentionDays int16   `gorm:"column:analytics_retention_days;not null;default:7" json:"analytics_retention_days"`
	LinkExpirationDays     *int16  `gorm:"column:link_expiration_days" json:"link_expiration_days,omitempty"` // NULL = never expires
	CustomAliases          bool    `gorm:"column:custom_aliases;not null;default:false" json:"custom_aliases"`
	PasswordProtectedLinks bool    `gorm:"column:password_protected_links;not null;default:false" json:"password_protected_links"`
	APIAccess              bool    `gorm:"column:api_access;not null;default:false" json:"api_access"`
	CustomDomains          bool    `gorm:"column:custom_domains;not null;default:false" json:"custom_domains"`
	PrioritySupport        bool    `gorm:"column:priority_support;not null;default:false" json:"priority_support"`
	CreatedAt              time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	IsActive               bool    `gorm:"column:is_active;not null;default:true" json:"is_active"`

	// Relationships
	Users []User `gorm:"foreignKey:SubscriptionTypeID" json:"users,omitempty"`
}

// TableName возвращает название таблицы для GORM
func (SubscriptionType) TableName() string {
	return "subscription_types"
}

// IsUnlimited проверяет, является ли подписка безлимитной по ссылкам
func (st *SubscriptionType) IsUnlimited() bool {
	return st.MaxLinksPerMonth == nil
}

// HasFeature проверяет, доступна ли определенная функция в подписке
func (st *SubscriptionType) HasFeature(feature string) bool {
	switch feature {
	case "custom_aliases":
		return st.CustomAliases
	case "password_protected_links":
		return st.PasswordProtectedLinks
	case "api_access":
		return st.APIAccess
	case "custom_domains":
		return st.CustomDomains
	case "priority_support":
		return st.PrioritySupport
	default:
		return false
	}
}