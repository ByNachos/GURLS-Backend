-- 003_create_links.sql
-- Создание таблицы ссылок

CREATE TABLE IF NOT EXISTS links (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) NOT NULL,
    original_url TEXT NOT NULL,
    alias VARCHAR(20) UNIQUE NOT NULL,
    title VARCHAR(200) NULL,
    description VARCHAR(500) NULL,
    expires_at TIMESTAMP WITH TIME ZONE NULL,
    max_clicks INTEGER NULL,
    click_count INTEGER NOT NULL DEFAULT 0,
    password_hash CHAR(60) NULL,  -- для защищенных ссылок
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    is_active BOOLEAN NOT NULL DEFAULT true
);

-- Индексы
CREATE UNIQUE INDEX idx_links_alias ON links(alias);
CREATE INDEX idx_links_user_id ON links(user_id);
CREATE INDEX idx_links_created_at ON links(created_at);
CREATE INDEX idx_links_expires_at ON links(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_links_active ON links(is_active) WHERE is_active = true;
CREATE INDEX idx_links_click_count ON links(click_count);