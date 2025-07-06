-- 005_create_user_stats.sql
-- Создание таблицы статистики пользователей

CREATE TABLE IF NOT EXISTS user_stats (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) UNIQUE NOT NULL,
    links_created_this_month INTEGER NOT NULL DEFAULT 0,
    clicks_received_this_month INTEGER NOT NULL DEFAULT 0,
    period_start DATE NOT NULL,  -- начало месячного периода
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Индексы
CREATE UNIQUE INDEX idx_user_stats_user_id ON user_stats(user_id);
CREATE INDEX idx_user_stats_period_start ON user_stats(period_start);
CREATE INDEX idx_user_stats_updated_at ON user_stats(updated_at);