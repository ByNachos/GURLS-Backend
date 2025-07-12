package domain

import (
	"time"
)

// Payment represents a payment transaction
type Payment struct {
	ID                   int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID               int64     `gorm:"column:user_id;not null" json:"user_id"`
	PaymentID            string    `gorm:"column:payment_id;uniqueIndex;not null" json:"payment_id"`
	Amount               float64   `gorm:"column:amount;type:decimal(10,2);not null" json:"amount"`
	Currency             string    `gorm:"column:currency;size:3;not null;default:'RUB'" json:"currency"`
	Status               string    `gorm:"column:status;size:50;not null" json:"status"`
	SubscriptionTypeID   int16     `gorm:"column:subscription_type_id" json:"subscription_type_id"`
	YookassaPaymentID    string    `gorm:"column:yookassa_payment_id" json:"yookassa_payment_id,omitempty"`
	YookassaPaymentData  string    `gorm:"column:yookassa_payment_data;type:text" json:"yookassa_payment_data,omitempty"`
	FailureReason        *string   `gorm:"column:failure_reason" json:"failure_reason,omitempty"`
	CreatedAt            time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	CompletedAt          *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`

	// Relationships
	User             *User             `gorm:"foreignKey:UserID" json:"user,omitempty"`
	SubscriptionType *SubscriptionType `gorm:"foreignKey:SubscriptionTypeID" json:"subscription_type,omitempty"`
}

// PaymentStatus represents different payment statuses
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusSucceeded PaymentStatus = "succeeded"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusCanceled  PaymentStatus = "canceled"
)

// TableName returns the table name for GORM
func (Payment) TableName() string {
	return "payments"
}

// IsCompleted checks if the payment is completed (succeeded or failed)
func (p *Payment) IsCompleted() bool {
	return p.Status == string(PaymentStatusSucceeded) || p.Status == string(PaymentStatusFailed)
}

// IsSuccessful checks if the payment is successful
func (p *Payment) IsSuccessful() bool {
	return p.Status == string(PaymentStatusSucceeded)
}

// SubscriptionChange represents a subscription change event
type SubscriptionChange struct {
	ID                  int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID              int64     `gorm:"column:user_id;not null" json:"user_id"`
	OldSubscriptionID   *int16    `gorm:"column:old_subscription_id" json:"old_subscription_id,omitempty"`
	NewSubscriptionID   int16     `gorm:"column:new_subscription_id;not null" json:"new_subscription_id"`
	PaymentID           *int64    `gorm:"column:payment_id" json:"payment_id,omitempty"`
	ChangeType          string    `gorm:"column:change_type;size:50;not null" json:"change_type"`
	EffectiveDate       time.Time `gorm:"column:effective_date;not null" json:"effective_date"`
	ExpirationDate      *time.Time `gorm:"column:expiration_date" json:"expiration_date,omitempty"`
	IsActive            bool      `gorm:"column:is_active;not null;default:true" json:"is_active"`
	CreatedAt           time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`

	// Relationships
	User                *User             `gorm:"foreignKey:UserID" json:"user,omitempty"`
	OldSubscription     *SubscriptionType `gorm:"foreignKey:OldSubscriptionID" json:"old_subscription,omitempty"`
	NewSubscription     *SubscriptionType `gorm:"foreignKey:NewSubscriptionID" json:"new_subscription,omitempty"`
	Payment             *Payment          `gorm:"foreignKey:PaymentID" json:"payment,omitempty"`
}

// SubscriptionChangeType represents different subscription change types
type SubscriptionChangeType string

const (
	SubscriptionChangeTypeUpgrade   SubscriptionChangeType = "upgrade"
	SubscriptionChangeTypeDowngrade SubscriptionChangeType = "downgrade"
	SubscriptionChangeTypeRenewal   SubscriptionChangeType = "renewal"
	SubscriptionChangeTypeExpired   SubscriptionChangeType = "expired"
)

// TableName returns the table name for GORM
func (SubscriptionChange) TableName() string {
	return "subscription_changes"
}

// YookassaWebhookPayload represents the payload from Yookassa webhook
type YookassaWebhookPayload struct {
	Type   string `json:"type"`
	Event  string `json:"event"`
	Object struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		Amount       struct {
			Value    string `json:"value"`
			Currency string `json:"currency"`
		} `json:"amount"`
		Description string `json:"description"`
		Metadata    struct {
			UserID               string `json:"user_id"`
			SubscriptionTypeID   string `json:"subscription_type_id"`
			PaymentID            string `json:"payment_id"`
		} `json:"metadata"`
		PaymentMethod struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"payment_method"`
		CreatedAt string `json:"created_at"`
		Test      bool   `json:"test"`
	} `json:"object"`
}

// PaymentRequest represents a payment creation request
type PaymentRequest struct {
	UserID               int64   `json:"user_id"`
	SubscriptionTypeID   int16   `json:"subscription_type_id"`
	Amount               float64 `json:"amount"`
	Currency             string  `json:"currency"`
	Description          string  `json:"description"`
	ReturnURL            string  `json:"return_url"`
}

// PaymentResponse represents a payment creation response
type PaymentResponse struct {
	PaymentID         string `json:"payment_id"`
	Status            string `json:"status"`
	Amount            float64 `json:"amount"`
	Currency          string `json:"currency"`
	ConfirmationURL   string `json:"confirmation_url"`
	CreatedAt         string `json:"created_at"`
}