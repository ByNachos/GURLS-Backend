-- migrate.sql
-- Полная миграция базы данных для GURLS Backend

-- Выполняем все миграции по порядку
\i 001_create_subscription_types.sql
\i 002_create_users.sql
\i 003_create_links.sql
\i 004_create_clicks.sql
\i 005_create_user_stats.sql
\i 006_create_sessions.sql
\i 007_create_refresh_tokens.sql

-- Информация о выполненных миграциях
SELECT 'Database migration completed successfully!' as status;