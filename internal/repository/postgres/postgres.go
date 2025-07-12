package postgres

import (
	"GURLS-Backend/internal/domain"
	"GURLS-Backend/internal/repository"
	"context"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// PostgresStorage реализует интерфейс Storage для PostgreSQL
type PostgresStorage struct {
	db  *gorm.DB
	log *zap.Logger
}

// New создает новый экземпляр PostgreSQL storage
func New(db *gorm.DB, log *zap.Logger) *PostgresStorage {
	return &PostgresStorage{
		db:  db,
		log: log,
	}
}

// --- User Methods (updated for web-only authentication) ---

// CreateUser создает нового пользователя с email и паролем
func (s *PostgresStorage) CreateUser(ctx context.Context, email, passwordHash string) (*domain.User, error) {
	user := domain.User{
		Email:              email,
		PasswordHash:       passwordHash,
		SubscriptionTypeID: 1, // default to 'free' plan
		IsActive:           true,
		EmailVerified:      false,
	}

	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		s.log.Error("failed to create user", zap.String("email", email), zap.Error(err))
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Создаем статистику для нового пользователя
	if err := s.createUserStats(ctx, user.ID); err != nil {
		s.log.Warn("failed to create user stats", zap.Int64("user_id", user.ID), zap.Error(err))
	}

	s.log.Info("created new user", zap.Int64("user_id", user.ID), zap.String("email", email))
	return &user, nil
}

// GetUserByEmail получает пользователя по email
func (s *PostgresStorage) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User

	err := s.db.WithContext(ctx).Where("email = ? AND is_active = ?", email, true).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		s.log.Error("failed to get user by email", zap.String("email", email), zap.Error(err))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByID получает пользователя по ID
func (s *PostgresStorage) GetUserByID(ctx context.Context, userID int64) (*domain.User, error) {
	var user domain.User

	err := s.db.WithContext(ctx).Where("id = ? AND is_active = ?", userID, true).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		s.log.Error("failed to get user by id", zap.Int64("user_id", userID), zap.Error(err))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// UpdateUser обновляет данные пользователя
func (s *PostgresStorage) UpdateUser(ctx context.Context, user *domain.User) error {
	err := s.db.WithContext(ctx).Save(user).Error
	if err != nil {
		s.log.Error("failed to update user", zap.Int64("user_id", user.ID), zap.Error(err))
		return fmt.Errorf("failed to update user: %w", err)
	}

	s.log.Info("updated user", zap.Int64("user_id", user.ID))
	return nil
}

// FindUserByEmailAndPassword находит пользователя для аутентификации
func (s *PostgresStorage) FindUserByEmailAndPassword(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User

	err := s.db.WithContext(ctx).Where("email = ? AND is_active = ?", email, true).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		s.log.Error("failed to find user for authentication", zap.String("email", email), zap.Error(err))
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	return &user, nil
}

// --- Link Methods ---

// SaveLink сохраняет новую ссылку
func (s *PostgresStorage) SaveLink(ctx context.Context, link *domain.Link) error {
	// Проверяем, существует ли уже такой алиас
	var existingLink domain.Link
	err := s.db.WithContext(ctx).Where("alias = ?", link.Alias).First(&existingLink).Error
	if err == nil {
		return repository.ErrAliasExists
	}
	if err != gorm.ErrRecordNotFound {
		s.log.Error("failed to check alias existence", zap.String("alias", link.Alias), zap.Error(err))
		return fmt.Errorf("failed to check alias: %w", err)
	}

	// Сохраняем ссылку
	if err := s.db.WithContext(ctx).Create(link).Error; err != nil {
		s.log.Error("failed to save link", zap.String("alias", link.Alias), zap.Error(err))
		return fmt.Errorf("failed to save link: %w", err)
	}

	// Обновляем статистику пользователя
	if err := s.incrementLinksCreated(ctx, link.UserID); err != nil {
		s.log.Warn("failed to update user stats", zap.Int64("user_id", link.UserID), zap.Error(err))
	}

	s.log.Info("saved new link", zap.String("alias", link.Alias), zap.Int64("user_id", link.UserID))
	return nil
}

// GetLink получает ссылку по алиасу
func (s *PostgresStorage) GetLink(ctx context.Context, alias string) (*domain.Link, error) {
	var link domain.Link

	err := s.db.WithContext(ctx).Where("alias = ? AND is_active = ?", alias, true).First(&link).Error
	if err == gorm.ErrRecordNotFound {
		return nil, repository.ErrAliasNotFound
	}
	if err != nil {
		s.log.Error("failed to get link", zap.String("alias", alias), zap.Error(err))
		return nil, fmt.Errorf("failed to get link: %w", err)
	}

	// Проверяем срок действия ссылки
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return nil, repository.ErrAliasNotFound
	}

	return &link, nil
}

// DeleteLink удаляет ссылку (мягкое удаление)
func (s *PostgresStorage) DeleteLink(ctx context.Context, alias string) error {
	result := s.db.WithContext(ctx).Model(&domain.Link{}).Where("alias = ?", alias).Update("is_active", false)
	if result.Error != nil {
		s.log.Error("failed to delete link", zap.String("alias", alias), zap.Error(result.Error))
		return fmt.Errorf("failed to delete link: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return repository.ErrAliasNotFound
	}

	s.log.Info("deleted link", zap.String("alias", alias))
	return nil
}

// AliasExists проверяет, существует ли алиас
func (s *PostgresStorage) AliasExists(ctx context.Context, alias string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&domain.Link{}).Where("alias = ?", alias).Count(&count).Error
	if err != nil {
		s.log.Error("failed to check alias existence", zap.String("alias", alias), zap.Error(err))
		return false, fmt.Errorf("failed to check alias: %w", err)
	}

	return count > 0, nil
}

// RecordClick записывает клик и обновляет статистику
func (s *PostgresStorage) RecordClick(ctx context.Context, alias string, deviceType string) error {
	return s.RecordClickAdvanced(ctx, alias, deviceType, nil, nil, nil, nil)
}

// RecordClickAdvanced записывает клик с расширенной информацией
func (s *PostgresStorage) RecordClickAdvanced(ctx context.Context, alias string, deviceType string, ipAddress *string, userAgent *string, referer *string, clickedAt *time.Time) error {
	// Начинаем транзакцию
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Получаем ссылку
	var link domain.Link
	err := tx.Where("alias = ? AND is_active = ?", alias, true).First(&link).Error
	if err == gorm.ErrRecordNotFound {
		tx.Rollback()
		return repository.ErrAliasNotFound
	}
	if err != nil {
		tx.Rollback()
		s.log.Error("failed to get link for click recording", zap.String("alias", alias), zap.Error(err))
		return fmt.Errorf("failed to get link: %w", err)
	}

	// Обновляем счетчик кликов
	err = tx.Model(&link).Update("click_count", gorm.Expr("click_count + 1")).Error
	if err != nil {
		tx.Rollback()
		s.log.Error("failed to update click count", zap.String("alias", alias), zap.Error(err))
		return fmt.Errorf("failed to update click count: %w", err)
	}

	// Создаем запись клика с расширенной информацией
	clickTime := time.Now()
	if clickedAt != nil {
		clickTime = *clickedAt
	}

	click := domain.Click{
		LinkID:     link.ID,
		DeviceType: &deviceType,
		UserAgent:  userAgent,
		Referer:    referer,
		ClickedAt:  clickTime,
		IsUnique:   true, // Пока считаем все клики уникальными
	}

	// Обработка IP адреса
	if ipAddress != nil && *ipAddress != "" {
		if ip := net.ParseIP(*ipAddress); ip != nil {
			click.IPAddress = &ip
		}
	}

	err = tx.Create(&click).Error
	if err != nil {
		tx.Rollback()
		s.log.Error("failed to create click record", zap.String("alias", alias), zap.Error(err))
		return fmt.Errorf("failed to create click: %w", err)
	}

	// Обновляем статистику пользователя
	err = s.incrementClicksReceived(tx, link.UserID)
	if err != nil {
		tx.Rollback()
		s.log.Error("failed to update user click stats", zap.Int64("user_id", link.UserID), zap.Error(err))
		return fmt.Errorf("failed to update user stats: %w", err)
	}

	// Коммитим транзакцию
	if err := tx.Commit().Error; err != nil {
		s.log.Error("failed to commit click transaction", zap.String("alias", alias), zap.Error(err))
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Info("recorded click", zap.String("alias", alias), zap.String("device_type", deviceType))
	return nil
}

// ListUserLinks возвращает список ссылок пользователя
func (s *PostgresStorage) ListUserLinks(ctx context.Context, userID int64) ([]*domain.Link, error) {
	var links []*domain.Link

	err := s.db.WithContext(ctx).Where("user_id = ? AND is_active = ?", userID, true).
		Order("created_at DESC").Find(&links).Error
	if err != nil {
		s.log.Error("failed to list user links", zap.Int64("user_id", userID), zap.Error(err))
		return nil, fmt.Errorf("failed to list user links: %w", err)
	}

	return links, nil
}

// GetClicksByDevice возвращает статистику кликов по типам устройств для ссылки
func (s *PostgresStorage) GetClicksByDevice(ctx context.Context, linkID int64) (map[string]int64, error) {
	var results []struct {
		DeviceType string `gorm:"column:device_type"`
		Count      int64  `gorm:"column:count"`
	}

	err := s.db.WithContext(ctx).
		Model(&domain.Click{}).
		Select("COALESCE(device_type, 'unknown') as device_type, count(*) as count").
		Where("link_id = ?", linkID).
		Group("device_type").
		Find(&results).Error

	if err != nil {
		s.log.Error("failed to get clicks by device", zap.Int64("link_id", linkID), zap.Error(err))
		return nil, fmt.Errorf("failed to get clicks by device: %w", err)
	}

	clicksByDevice := make(map[string]int64)
	for _, result := range results {
		clicksByDevice[result.DeviceType] = result.Count
	}

	return clicksByDevice, nil
}

// GetLinkAndRecordClick получает ссылку и записывает клик атомарно (для unified service)
func (s *PostgresStorage) GetLinkAndRecordClick(ctx context.Context, alias string, ipAddress *string, userAgent *string, referer *string) (*domain.Link, error) {
	// Начинаем транзакцию
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Получаем ссылку
	var link domain.Link
	err := tx.Where("alias = ? AND is_active = ?", alias, true).First(&link).Error
	if err == gorm.ErrRecordNotFound {
		tx.Rollback()
		return nil, repository.ErrAliasNotFound
	}
	if err != nil {
		tx.Rollback()
		s.log.Error("failed to get link for redirect", zap.String("alias", alias), zap.Error(err))
		return nil, fmt.Errorf("failed to get link: %w", err)
	}

	// Проверяем срок действия ссылки
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		tx.Rollback()
		return nil, repository.ErrAliasNotFound
	}

	// Обновляем счетчик кликов
	err = tx.Model(&link).Update("click_count", gorm.Expr("click_count + 1")).Error
	if err != nil {
		tx.Rollback()
		s.log.Error("failed to update click count", zap.String("alias", alias), zap.Error(err))
		return nil, fmt.Errorf("failed to update click count: %w", err)
	}

	// Создаем запись клика
	clickedAt := time.Now()
	click := domain.Click{
		LinkID:     link.ID,
		ClickedAt:  clickedAt,
		IsUnique:   true, // Simplified logic for now
	}

	// Добавляем дополнительную информацию если есть
	if ipAddress != nil {
		if ip := net.ParseIP(*ipAddress); ip != nil {
			click.IPAddress = &ip
		}
	}
	if userAgent != nil {
		click.UserAgent = userAgent
	}
	if referer != nil {
		click.Referer = referer
	}

	err = tx.Create(&click).Error
	if err != nil {
		tx.Rollback()
		s.log.Error("failed to record click", zap.String("alias", alias), zap.Error(err))
		return nil, fmt.Errorf("failed to record click: %w", err)
	}

	// Обновляем статистику пользователя
	if err := s.incrementClicksReceived(tx, link.UserID); err != nil {
		s.log.Warn("failed to update user click stats", zap.Int64("user_id", link.UserID), zap.Error(err))
	}

	// Коммитим транзакцию
	if err := tx.Commit().Error; err != nil {
		s.log.Error("failed to commit transaction", zap.String("alias", alias), zap.Error(err))
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Info("processed redirect with analytics", zap.String("alias", alias), zap.Int64("link_id", link.ID))
	return &link, nil
}

// --- Helper Methods ---

// createUserStats создает начальную статистику для пользователя
func (s *PostgresStorage) createUserStats(ctx context.Context, userID int64) error {
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	stats := domain.UserStats{
		UserID:                  userID,
		LinksCreatedThisMonth:   0,
		ClicksReceivedThisMonth: 0,
		PeriodStart:             periodStart,
	}

	return s.db.WithContext(ctx).Create(&stats).Error
}

// incrementLinksCreated увеличивает счетчик созданных ссылок
func (s *PostgresStorage) incrementLinksCreated(ctx context.Context, userID int64) error {
	return s.db.WithContext(ctx).Model(&domain.UserStats{}).
		Where("user_id = ?", userID).
		Update("links_created_this_month", gorm.Expr("links_created_this_month + 1")).Error
}

// incrementClicksReceived увеличивает счетчик полученных кликов
func (s *PostgresStorage) incrementClicksReceived(tx *gorm.DB, userID int64) error {
	return tx.Model(&domain.UserStats{}).
		Where("user_id = ?", userID).
		Update("clicks_received_this_month", gorm.Expr("clicks_received_this_month + 1")).Error
}

// --- Payment Methods ---

// CreatePayment creates a new payment record
func (s *PostgresStorage) CreatePayment(ctx context.Context, payment *domain.Payment) error {
	if err := s.db.WithContext(ctx).Create(payment).Error; err != nil {
		s.log.Error("failed to create payment", 
			zap.String("payment_id", payment.PaymentID), 
			zap.Int64("user_id", payment.UserID), 
			zap.Error(err))
		return fmt.Errorf("failed to create payment: %w", err)
	}
	return nil
}

// GetPaymentByID retrieves a payment by its internal payment ID
func (s *PostgresStorage) GetPaymentByID(ctx context.Context, paymentID string) (*domain.Payment, error) {
	var payment domain.Payment
	if err := s.db.WithContext(ctx).Where("payment_id = ?", paymentID).First(&payment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, repository.ErrPaymentNotFound
		}
		s.log.Error("failed to get payment by ID", zap.String("payment_id", paymentID), zap.Error(err))
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}
	return &payment, nil
}

// GetPaymentByYooKassaID retrieves a payment by YooKassa payment ID
func (s *PostgresStorage) GetPaymentByYooKassaID(ctx context.Context, yookassaID string) (*domain.Payment, error) {
	var payment domain.Payment
	if err := s.db.WithContext(ctx).Where("yookassa_payment_id = ?", yookassaID).First(&payment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, repository.ErrPaymentNotFound
		}
		s.log.Error("failed to get payment by YooKassa ID", zap.String("yookassa_id", yookassaID), zap.Error(err))
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}
	return &payment, nil
}

// UpdatePayment updates an existing payment record
func (s *PostgresStorage) UpdatePayment(ctx context.Context, payment *domain.Payment) error {
	if err := s.db.WithContext(ctx).Save(payment).Error; err != nil {
		s.log.Error("failed to update payment", 
			zap.String("payment_id", payment.PaymentID), 
			zap.Error(err))
		return fmt.Errorf("failed to update payment: %w", err)
	}
	return nil
}

// ListUserPayments retrieves all payments for a specific user
func (s *PostgresStorage) ListUserPayments(ctx context.Context, userID int64) ([]*domain.Payment, error) {
	var payments []*domain.Payment
	if err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&payments).Error; err != nil {
		s.log.Error("failed to list user payments", zap.Int64("user_id", userID), zap.Error(err))
		return nil, fmt.Errorf("failed to list user payments: %w", err)
	}
	return payments, nil
}

// --- Subscription Methods ---

// GetSubscriptionType retrieves a subscription type by ID
func (s *PostgresStorage) GetSubscriptionType(ctx context.Context, id int16) (*domain.SubscriptionType, error) {
	var subscriptionType domain.SubscriptionType
	if err := s.db.WithContext(ctx).Where("id = ? AND is_active = ?", id, true).First(&subscriptionType).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, repository.ErrSubscriptionTypeNotFound
		}
		s.log.Error("failed to get subscription type", zap.Int16("id", id), zap.Error(err))
		return nil, fmt.Errorf("failed to get subscription type: %w", err)
	}
	return &subscriptionType, nil
}

// ListSubscriptionTypes retrieves all active subscription types
func (s *PostgresStorage) ListSubscriptionTypes(ctx context.Context) ([]*domain.SubscriptionType, error) {
	var subscriptionTypes []*domain.SubscriptionType
	if err := s.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("price_monthly ASC").
		Find(&subscriptionTypes).Error; err != nil {
		s.log.Error("failed to list subscription types", zap.Error(err))
		return nil, fmt.Errorf("failed to list subscription types: %w", err)
	}
	return subscriptionTypes, nil
}

// CreateSubscriptionChange creates a new subscription change record
func (s *PostgresStorage) CreateSubscriptionChange(ctx context.Context, change *domain.SubscriptionChange) error {
	if err := s.db.WithContext(ctx).Create(change).Error; err != nil {
		s.log.Error("failed to create subscription change", 
			zap.Int64("user_id", change.UserID), 
			zap.Int16("new_subscription", change.NewSubscriptionID), 
			zap.Error(err))
		return fmt.Errorf("failed to create subscription change: %w", err)
	}
	return nil
}

// GetActiveSubscriptionChanges retrieves active subscription changes for a user
func (s *PostgresStorage) GetActiveSubscriptionChanges(ctx context.Context, userID int64) ([]*domain.SubscriptionChange, error) {
	var changes []*domain.SubscriptionChange
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND is_active = ?", userID, true).
		Order("created_at DESC").
		Find(&changes).Error; err != nil {
		s.log.Error("failed to get active subscription changes", zap.Int64("user_id", userID), zap.Error(err))
		return nil, fmt.Errorf("failed to get active subscription changes: %w", err)
	}
	return changes, nil
}