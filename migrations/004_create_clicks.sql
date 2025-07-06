-- 004_create_clicks.sql
-- Создание таблицы кликов (аналитика)

CREATE TABLE IF NOT EXISTS clicks (
    id BIGSERIAL PRIMARY KEY,
    link_id BIGINT REFERENCES links(id) NOT NULL,
    ip_address INET NULL,
    user_agent TEXT NULL,
    referer VARCHAR(500) NULL,
    country CHAR(2) NULL,  -- ISO код страны
    city VARCHAR(100) NULL,
    device_type VARCHAR(10) NULL CHECK (device_type IN ('desktop', 'mobile', 'tablet', 'unknown')),
    browser VARCHAR(50) NULL,
    os VARCHAR(50) NULL,
    clicked_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    is_unique BOOLEAN NOT NULL DEFAULT true  -- уникальный клик от IP за день
);

-- Индексы
CREATE INDEX idx_clicks_link_id ON clicks(link_id);
CREATE INDEX idx_clicks_clicked_at ON clicks(clicked_at);
CREATE INDEX idx_clicks_ip_address ON clicks(ip_address) WHERE ip_address IS NOT NULL;
CREATE INDEX idx_clicks_device_type ON clicks(device_type) WHERE device_type IS NOT NULL;
CREATE INDEX idx_clicks_country ON clicks(country) WHERE country IS NOT NULL;

-- Композитный индекс для проверки уникальности кликов по IP за день
CREATE INDEX idx_clicks_unique_daily ON clicks(link_id, ip_address, date(clicked_at)) WHERE is_unique = true;