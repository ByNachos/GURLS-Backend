package database

import (
	"GURLS-Backend/internal/domain"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AutoMigrate выполняет автоматические миграции для всех доменных моделей
func AutoMigrate(db *gorm.DB, log *zap.Logger) error {
	log.Info("starting database auto-migration")

	// Порядок миграций важен из-за внешних ключей
	models := []interface{}{
		&domain.SubscriptionType{}, // Сначала справочники
		&domain.User{},             // Затем пользователи
		&domain.Link{},             // Ссылки (зависят от пользователей)
		&domain.Click{},            // Клики (зависят от ссылок)
		&domain.UserStats{},        // Статистика (зависит от пользователей)
		&domain.Session{},          // Сессии (зависят от пользователей)
		&domain.RefreshToken{},     // JWT токены (зависят от пользователей)
	}

	log.Info("migrating database models", zap.Int("total_models", len(models)))

	for i, model := range models {
		modelName := fmt.Sprintf("%T", model)
		log.Info("migrating model", 
			zap.String("model", modelName),
			zap.Int("step", i+1),
			zap.Int("total", len(models)))

		if err := db.AutoMigrate(model); err != nil {
			log.Error("failed to migrate model", 
				zap.String("model", modelName),
				zap.Error(err))
			return fmt.Errorf("failed to migrate model %s: %w", modelName, err)
		}
		
		log.Info("model migrated successfully", zap.String("model", modelName))
	}

	log.Info("database auto-migration completed successfully", zap.Int("migrated_models", len(models)))
	return nil
}

// SeedData заполняет базу данных начальными данными
func SeedData(db *gorm.DB, log *zap.Logger) error {
	log.Info("starting database seeding")

	// Проверяем, есть ли уже данные
	var count int64
	db.Model(&domain.SubscriptionType{}).Count(&count)
	if count > 0 {
		log.Info("subscription types already exist, skipping seeding", zap.Int64("existing_count", count))
		return nil
	}

	log.Info("no existing subscription types found, proceeding with seeding")

	// Создаем типы подписок
	subscriptionTypes := []domain.SubscriptionType{
		{
			Name:                    "free",
			DisplayName:             "Free Plan",
			PriceMonthly:            0.00,
			PriceYearly:             0.00,
			MaxLinksPerMonth:        toInt(10),
			MaxClicksPerMonth:       toInt(500),
			AnalyticsRetentionDays:  7,
			LinkExpirationDays:      toInt16(30),
			CustomAliases:           false,
			PasswordProtectedLinks:  false,
			APIAccess:               false,
			CustomDomains:           false,
			PrioritySupport:         false,
			IsActive:                true,
		},
		{
			Name:                    "base",
			DisplayName:             "Base Plan",
			PriceMonthly:            9.99,
			PriceYearly:             99.99,
			MaxLinksPerMonth:        toInt(100),
			MaxClicksPerMonth:       toInt(5000),
			AnalyticsRetentionDays:  30,
			LinkExpirationDays:      toInt16(365),
			CustomAliases:           true,
			PasswordProtectedLinks:  false,
			APIAccess:               false,
			CustomDomains:           false,
			PrioritySupport:         false,
			IsActive:                true,
		},
		{
			Name:                    "enterprise",
			DisplayName:             "Enterprise Plan",
			PriceMonthly:            49.99,
			PriceYearly:             499.99,
			MaxLinksPerMonth:        nil, // unlimited
			MaxClicksPerMonth:       nil, // unlimited
			AnalyticsRetentionDays:  365,
			LinkExpirationDays:      nil, // never expires
			CustomAliases:           true,
			PasswordProtectedLinks:  true,
			APIAccess:               true,
			CustomDomains:           true,
			PrioritySupport:         true,
			IsActive:                true,
		},
	}

	log.Info("creating subscription types", zap.Int("types_count", len(subscriptionTypes)))
	
	if err := db.Create(&subscriptionTypes).Error; err != nil {
		log.Error("failed to seed subscription types", zap.Error(err))
		return fmt.Errorf("failed to seed subscription types: %w", err)
	}

	log.Info("database seeding completed successfully", zap.Int("subscription_types_created", len(subscriptionTypes)))
	return nil
}

// toInt возвращает указатель на int - хелпер для создания nullable полей
func toInt(val int) *int {
	return &val
}

// toInt16 возвращает указатель на int16 - хелпер для создания nullable полей
func toInt16(val int16) *int16 {
	return &val
}