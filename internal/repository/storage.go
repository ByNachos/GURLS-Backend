package repository

import (
	"GURLS-Backend/internal/domain"
	"context"
	"errors"
	"time"
)

var (
	ErrAliasNotFound              = errors.New("alias not found")
	ErrAliasExists                = errors.New("alias already exists")
	ErrPaymentNotFound            = errors.New("payment not found")
	ErrSubscriptionTypeNotFound   = errors.New("subscription type not found")
)

type Storage interface {
	// User methods (updated for web-only authentication)
	CreateUser(ctx context.Context, email, passwordHash string) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserByID(ctx context.Context, userID int64) (*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) error

	// Authentication methods
	FindUserByEmailAndPassword(ctx context.Context, email string) (*domain.User, error)

	// Link methods
	SaveLink(ctx context.Context, link *domain.Link) error
	GetLink(ctx context.Context, alias string) (*domain.Link, error)
	DeleteLink(ctx context.Context, alias string) error
	AliasExists(ctx context.Context, alias string) (bool, error)
	RecordClick(ctx context.Context, alias string, deviceType string) error
	ListUserLinks(ctx context.Context, userID int64) ([]*domain.Link, error)

	// Extended analytics methods
	RecordClickAdvanced(ctx context.Context, alias string, deviceType string, ipAddress *string, userAgent *string, referer *string, clickedAt *time.Time) error
	GetClicksByDevice(ctx context.Context, linkID int64) (map[string]int64, error)
	
	// Redirect with analytics recording (for unified service)
	GetLinkAndRecordClick(ctx context.Context, alias string, ipAddress *string, userAgent *string, referer *string) (*domain.Link, error)

	// Payment methods
	CreatePayment(ctx context.Context, payment *domain.Payment) error
	GetPaymentByID(ctx context.Context, paymentID string) (*domain.Payment, error)
	GetPaymentByYooKassaID(ctx context.Context, yookassaID string) (*domain.Payment, error)
	UpdatePayment(ctx context.Context, payment *domain.Payment) error
	ListUserPayments(ctx context.Context, userID int64) ([]*domain.Payment, error)

	// Subscription methods
	GetSubscriptionType(ctx context.Context, id int16) (*domain.SubscriptionType, error)
	ListSubscriptionTypes(ctx context.Context) ([]*domain.SubscriptionType, error)
	CreateSubscriptionChange(ctx context.Context, change *domain.SubscriptionChange) error
	GetActiveSubscriptionChanges(ctx context.Context, userID int64) ([]*domain.SubscriptionChange, error)
}
