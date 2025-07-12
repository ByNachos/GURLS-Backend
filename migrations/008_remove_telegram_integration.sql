-- 008_remove_telegram_integration.sql
-- Удаление Telegram интеграции из системы GURLS
-- Переход к web-only аутентификации

-- Удаляем Telegram специфичные поля из users таблицы
ALTER TABLE users DROP COLUMN IF EXISTS telegram_id;
ALTER TABLE users DROP COLUMN IF EXISTS telegram_username;
ALTER TABLE users DROP COLUMN IF EXISTS registration_source;

-- Делаем email и password_hash обязательными для web-only пользователей
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;

-- Добавляем constraint для валидации email формата
ALTER TABLE users ADD CONSTRAINT users_email_check 
    CHECK (email ~ '^[^@]+@[^@]+\.[^@]+$');

-- Удаляем старые индексы связанные с Telegram
DROP INDEX IF EXISTS idx_users_telegram_id;

-- Обновляем существующие записи если есть (для dev/test среды)
-- В production этот блок нужно адаптировать под реальные данные
UPDATE users 
SET 
    email = COALESCE(email, 'user' || id || '@gurls.local'),
    password_hash = COALESCE(password_hash, '$2a$12$dummy.hash.for.migration.purpose.only')
WHERE email IS NULL OR password_hash IS NULL;

-- Добавляем индекс для быстрого поиска по email
CREATE INDEX IF NOT EXISTS idx_users_email_active ON users(email) WHERE is_active = true;

-- Комментарий для документации
COMMENT ON TABLE users IS 'Пользователи системы GURLS (только web аутентификация)';
COMMENT ON COLUMN users.email IS 'Email пользователя (обязательное поле для web auth)';
COMMENT ON COLUMN users.password_hash IS 'Хеш пароля bcrypt (обязательное поле для web auth)';