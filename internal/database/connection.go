package database

import (
	"GURLS-Backend/internal/config"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewConnection создает новое подключение к PostgreSQL с помощью GORM
func NewConnection(cfg *config.Database, log *zap.Logger) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
		cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port, cfg.SSLMode, cfg.Timezone)

	// Настраиваем GORM logger
	gormLogger := logger.Default
	if log != nil {
		gormLogger = logger.Default.LogMode(logger.Info)
	}

	// Открываем соединение
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Настраиваем пул соединений
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)

	connMaxLifetime, err := time.ParseDuration(cfg.ConnMaxLifetime)
	if err != nil {
		log.Warn("failed to parse conn_max_lifetime, using default 1h", zap.Error(err))
		connMaxLifetime = time.Hour
	}
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	// Проверяем подключение
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("successfully connected to PostgreSQL database",
		zap.String("host", cfg.Host),
		zap.String("database", cfg.DBName),
		zap.Int("max_idle_conns", cfg.MaxIdleConns),
		zap.Int("max_open_conns", cfg.MaxOpenConns))

	return db, nil
}

// Close закрывает соединение с базой данных
func Close(db *gorm.DB, log *zap.Logger) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB instance: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	log.Info("database connection closed")
	return nil
}

// HealthCheck проверяет состояние подключения к базе данных
func HealthCheck(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB instance: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}