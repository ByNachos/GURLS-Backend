-- 007_create_refresh_tokens.sql
-- Создание таблицы JWT refresh токенов

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) NOT NULL,
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    is_revoked BOOLEAN NOT NULL DEFAULT false,
    user_agent TEXT NULL,
    ip_address INET NULL,
    last_used_at TIMESTAMP WITH TIME ZONE NULL
);

-- Индексы
CREATE UNIQUE INDEX idx_refresh_tokens_token ON refresh_tokens(token);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
CREATE INDEX idx_refresh_tokens_is_revoked ON refresh_tokens(is_revoked) WHERE is_revoked = false;
CREATE INDEX idx_refresh_tokens_created_at ON refresh_tokens(created_at);
CREATE INDEX idx_refresh_tokens_last_used_at ON refresh_tokens(last_used_at) WHERE last_used_at IS NOT NULL;