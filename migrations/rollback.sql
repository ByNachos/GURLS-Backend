-- rollback.sql
-- Откат всех изменений (для тестирования)

-- Удаляем таблицы в обратном порядке (из-за внешних ключей)
DROP TABLE IF EXISTS subscription_changes CASCADE;
DROP TABLE IF EXISTS payments CASCADE;
DROP TABLE IF EXISTS refresh_tokens CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS user_stats CASCADE;
DROP TABLE IF EXISTS clicks CASCADE;
DROP TABLE IF EXISTS links CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS subscription_types CASCADE;

SELECT 'Database rollback completed!' as status;