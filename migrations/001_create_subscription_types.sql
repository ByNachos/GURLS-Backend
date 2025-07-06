-- 001_create_subscription_types.sql
-- Создание таблицы типов подписок

CREATE TABLE IF NOT EXISTS subscription_types (
    id SMALLSERIAL PRIMARY KEY,
    name VARCHAR(20) UNIQUE NOT NULL,
    display_name VARCHAR(50) NOT NULL,
    price_monthly DECIMAL(6,2) NOT NULL DEFAULT 0.00,
    price_yearly DECIMAL(7,2) NOT NULL DEFAULT 0.00,
    max_links_per_month INTEGER NULL,  -- NULL = unlimited
    max_clicks_per_month INTEGER NULL,  -- NULL = unlimited
    analytics_retention_days SMALLINT NOT NULL DEFAULT 7,
    link_expiration_days SMALLINT NULL,  -- NULL = never expires
    custom_aliases BOOLEAN NOT NULL DEFAULT false,
    password_protected_links BOOLEAN NOT NULL DEFAULT false,
    api_access BOOLEAN NOT NULL DEFAULT false,
    custom_domains BOOLEAN NOT NULL DEFAULT false,
    priority_support BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    is_active BOOLEAN NOT NULL DEFAULT true
);

-- Индексы
CREATE INDEX idx_subscription_types_name ON subscription_types(name);
CREATE INDEX idx_subscription_types_active ON subscription_types(is_active);

-- Начальные данные
INSERT INTO subscription_types (
    name, display_name, price_monthly, price_yearly, 
    max_links_per_month, max_clicks_per_month, analytics_retention_days, 
    link_expiration_days, custom_aliases, password_protected_links, 
    api_access, custom_domains, priority_support, is_active
) VALUES
    ('free', 'Free Plan', 0.00, 0.00, 10, 500, 7, 30, false, false, false, false, false, true),
    ('base', 'Base Plan', 9.99, 99.99, 100, 5000, 30, 365, true, false, false, false, false, true),
    ('enterprise', 'Enterprise Plan', 49.99, 499.99, NULL, NULL, 365, NULL, true, true, true, true, true, true)
ON CONFLICT (name) DO NOTHING;