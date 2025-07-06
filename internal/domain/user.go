package domain

import "time"

// User представляет пользователя сервиса.
type User struct {
	ID                    int64     `gorm:"primaryKey;column:id" json:"id"`
	Email                 *string   `gorm:"column:email;uniqueIndex" json:"email,omitempty"`
	TelegramID            *int64    `gorm:"column:telegram_id;uniqueIndex" json:"telegram_id,omitempty"`
	Username              *string   `gorm:"column:username" json:"username,omitempty"`
	FirstName             *string   `gorm:"column:first_name" json:"first_name,omitempty"`
	LastName              *string   `gorm:"column:last_name" json:"last_name,omitempty"`
	PasswordHash          *string   `gorm:"column:password_hash" json:"-"` // скрываем пароль в JSON
	RegistrationSource    string    `gorm:"column:registration_source;not null" json:"registration_source"`
	EmailVerified         bool      `gorm:"column:email_verified;default:false" json:"email_verified"`
	SubscriptionTypeID    int16     `gorm:"column:subscription_type_id;default:1" json:"subscription_type_id"`
	SubscriptionExpiresAt *time.Time `gorm:"column:subscription_expires_at" json:"subscription_expires_at,omitempty"`
	EmailVerificationToken *string   `gorm:"column:email_verification_token" json:"-"` // токен для подтверждения email
	PasswordResetToken     *string   `gorm:"column:password_reset_token" json:"-"` // токен для сброса пароля
	PasswordResetExpiresAt *time.Time `gorm:"column:password_reset_expires_at" json:"-"` // срок действия токена сброса
	LastLoginAt            *time.Time `gorm:"column:last_login_at" json:"last_login_at,omitempty"`
	CreatedAt              time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	IsActive               bool      `gorm:"column:is_active;default:true" json:"is_active"`

	// Relationships
	SubscriptionType *SubscriptionType `gorm:"foreignKey:SubscriptionTypeID" json:"subscription_type,omitempty"`
	Links           []Link            `gorm:"foreignKey:UserID" json:"links,omitempty"`
	UserStats       *UserStats        `gorm:"foreignKey:UserID" json:"user_stats,omitempty"`
	Sessions        []Session         `gorm:"foreignKey:UserID" json:"sessions,omitempty"`
	RefreshTokens   []RefreshToken    `gorm:"foreignKey:UserID" json:"refresh_tokens,omitempty"`
}

// TableName возвращает название таблицы для GORM
func (User) TableName() string {
	return "users"
}

// Backward compatibility fields and methods for existing code
// TgID возвращает Telegram ID для обратной совместимости
func (u *User) TgID() int64 {
	if u.TelegramID != nil {
		return *u.TelegramID
	}
	return 0
}

// SetTgID устанавливает Telegram ID для обратной совместимости  
func (u *User) SetTgID(tgID int64) {
	u.TelegramID = &tgID
}
