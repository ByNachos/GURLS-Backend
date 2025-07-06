-- 002_create_users.sql
-- Создание таблицы пользователей

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NULL,
    telegram_id BIGINT UNIQUE NULL,
    username VARCHAR(100) NULL,
    first_name VARCHAR(100) NULL,
    last_name VARCHAR(100) NULL,
    password_hash CHAR(60) NULL,
    registration_source VARCHAR(10) NOT NULL CHECK (registration_source IN ('telegram', 'web')),
    email_verified BOOLEAN NOT NULL DEFAULT false,
    subscription_type_id SMALLINT REFERENCES subscription_types(id) NOT NULL DEFAULT 1,
    subscription_expires_at TIMESTAMP WITH TIME ZONE NULL,
    email_verification_token VARCHAR(255) NULL,
    password_reset_token VARCHAR(255) NULL,
    password_reset_expires_at TIMESTAMP WITH TIME ZONE NULL,
    last_login_at TIMESTAMP WITH TIME ZONE NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    is_active BOOLEAN NOT NULL DEFAULT true,
    
    -- Ограничение: либо telegram_id, либо email+password должны быть заполнены
    CONSTRAINT users_auth_check CHECK (
        (registration_source = 'telegram' AND telegram_id IS NOT NULL) OR
        (registration_source = 'web' AND email IS NOT NULL AND password_hash IS NOT NULL)
    )
);

-- Индексы
CREATE UNIQUE INDEX idx_users_telegram_id ON users(telegram_id) WHERE telegram_id IS NOT NULL;
CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL;
CREATE INDEX idx_users_subscription_type_id ON users(subscription_type_id);
CREATE INDEX idx_users_created_at ON users(created_at);
CREATE INDEX idx_users_active ON users(is_active) WHERE is_active = true;
CREATE INDEX idx_users_email_verification_token ON users(email_verification_token) WHERE email_verification_token IS NOT NULL;
CREATE INDEX idx_users_password_reset_token ON users(password_reset_token) WHERE password_reset_token IS NOT NULL;
CREATE INDEX idx_users_last_login_at ON users(last_login_at);