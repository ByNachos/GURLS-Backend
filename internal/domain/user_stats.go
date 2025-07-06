package domain

import "time"

// UserStats представляет статистику использования пользователем
type UserStats struct {
	ID                       int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID                   int64     `gorm:"column:user_id;uniqueIndex;not null" json:"user_id"`
	LinksCreatedThisMonth    int       `gorm:"column:links_created_this_month;not null;default:0" json:"links_created_this_month"`
	ClicksReceivedThisMonth  int       `gorm:"column:clicks_received_this_month;not null;default:0" json:"clicks_received_this_month"`
	PeriodStart              time.Time `gorm:"column:period_start;type:date;not null" json:"period_start"` // начало месячного периода
	UpdatedAt                time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`

	// Relationships
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName возвращает название таблицы для GORM
func (UserStats) TableName() string {
	return "user_stats"
}

// CanCreateLink проверяет, может ли пользователь создать новую ссылку
func (us *UserStats) CanCreateLink(subscriptionType *SubscriptionType) bool {
	if subscriptionType.MaxLinksPerMonth == nil {
		return true // unlimited
	}
	return us.LinksCreatedThisMonth < *subscriptionType.MaxLinksPerMonth
}

// CanReceiveClick проверяет, может ли пользователь получить новый клик
func (us *UserStats) CanReceiveClick(subscriptionType *SubscriptionType) bool {
	if subscriptionType.MaxClicksPerMonth == nil {
		return true // unlimited
	}
	return us.ClicksReceivedThisMonth < *subscriptionType.MaxClicksPerMonth
}

// IsNewPeriod проверяет, начался ли новый месячный период
func (us *UserStats) IsNewPeriod() bool {
	now := time.Now()
	currentPeriodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return us.PeriodStart.Before(currentPeriodStart)
}

// ResetForNewPeriod сбрасывает статистику для нового месячного периода
func (us *UserStats) ResetForNewPeriod() {
	now := time.Now()
	us.PeriodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	us.LinksCreatedThisMonth = 0
	us.ClicksReceivedThisMonth = 0
}