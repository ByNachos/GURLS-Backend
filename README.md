# GURLS-Backend

Основной gRPC backend сервис для системы сокращения URL GURLS.

## Описание

GURLS-Backend предоставляет core бизнес-логику для управления короткими ссылками через gRPC API. Сервис обрабатывает создание, получение статистики, удаление ссылок и управление пользователями.

## gRPC API

Сервис предоставляет следующие методы:

- `CreateLink` - Создание новой короткой ссылки
- `GetLinkStats` - Получение статистики по ссылке  
- `DeleteLink` - Удаление ссылки
- `ListUserLinks` - Список ссылок пользователя

## Запуск

### Локальная разработка

```bash
# Установка зависимостей
go mod tidy

# Запуск сервера
go run ./cmd/backend

# Или сборка и запуск
go build -o bin/backend ./cmd/backend
./bin/backend
```

### Конфигурация

Сервис использует файл `config/local.yml` или переменные окружения:

- `GRPC_SERVER_PORT` - порт gRPC сервера (по умолчанию: 50051)
- `ALIAS_LENGTH` - длина генерируемых алиасов (по умолчанию: 4)
- `ENV` - окружение (local/dev/production)

### Docker

```bash
# Сборка
docker build -t gurls-backend .

# Запуск
docker run -p 50051:50051 gurls-backend
```

## Архитектура

- `cmd/backend/` - точка входа приложения
- `internal/grpc/server/` - gRPC сервер и обработчики
- `internal/service/` - бизнес-логика
- `internal/repository/` - слой данных
- `internal/domain/` - доменные модели
- `internal/config/` - конфигурация
- `pkg/` - переиспользуемые утилиты
- `api/proto/` - protobuf определения
- `gen/` - сгенерированный код

## Разработка

### Обновление gRPC контрактов

```bash
protoc -I./api/proto --go_out=. --go-grpc_out=. api/proto/v1/shortener.proto
```

### Тестирование

```bash
go test -v ./...
```

## Управление миграциями

### Новые параметры конфигурации

В конфигурационных файлах добавлены параметры для управления миграциями:

```yaml
database:
  # ... другие параметры базы данных
  # Migration settings
  auto_migrate: true   # Автоматически запускать миграции при старте
  seed_data: true      # Заполнять базу начальными данными
```

**Окружения:**
- `config/local.yml` - `auto_migrate: true, seed_data: true` (для разработки)
- `config/production.yml` - `auto_migrate: false, seed_data: false` (для продакшена)

**Переменные окружения:**
- `DATABASE_AUTO_MIGRATE` - включить/выключить автоматические миграции
- `DATABASE_SEED_DATA` - включить/выключить заполнение начальными данными

### Поведение системы

**Локальная разработка:**
- Миграции запускаются автоматически при старте
- База заполняется начальными данными (типы подписок)
- Удобно для быстрого развертывания

**Продакшн:**
- Миграции пропускаются при старте приложения
- Таблицы создаются вручную один раз из SQL файлов
- Безопасность и контроль над изменениями схемы

## База данных

### Архитектура базы данных

**Таблицы:**

1. **subscription_types** - Типы подписок (free, base, enterprise)
2. **users** - Пользователи (Telegram и веб-регистрация) + JWT поддержка
3. **links** - Сокращенные ссылки
4. **clicks** - Аналитика кликов
5. **user_stats** - Статистика использования пользователями
6. **sessions** - Веб-сессии пользователей (старый механизм)
7. **refresh_tokens** - JWT refresh токены для веб-авторизации

**Связи:**

```
subscription_types (1) ──── (N) users
users (1) ──── (N) links
users (1) ──── (1) user_stats
users (1) ──── (N) sessions
users (1) ──── (N) refresh_tokens
links (1) ──── (N) clicks
```

### Настройка локальной разработки

**1. Установка PostgreSQL**

*macOS (Homebrew):*
```bash
brew install postgresql@16
brew services start postgresql@16
```

*Ubuntu/Debian:*
```bash
sudo apt update
sudo apt install postgresql postgresql-contrib
sudo systemctl start postgresql
sudo systemctl enable postgresql
```

**2. Создание пользователя и базы данных**

```bash
# Подключение к PostgreSQL
sudo -u postgres psql

# Создание пользователя
CREATE USER postgres WITH PASSWORD 'your-secure-password';

# Создание базы данных
CREATE DATABASE gurls_test OWNER postgres;

# Выдача привилегий
GRANT ALL PRIVILEGES ON DATABASE gurls_test TO postgres;

# Выход
\q
```

**3. Конфигурация для разработки**

Создайте файл `.env` в корне проекта:

```env
# Database Configuration (для локального тестирования)
DATABASE_PASSWORD=your-secure-password

# Environment
ENV=local
```

### Развертывание на продакшн сервере

**Информация о сервере:**
- **Host**: your-production-server.com
- **Port**: 5432
- **User**: your-db-user
- **Database**: gurls
- **SSL**: require

⚠️ **ВАЖНО**: Никогда не указывайте реальные продакшн данные в документации!

**Применение миграций на сервере:**

```bash
# Подключение к серверу
psql -h your-production-server.com -p 5432 -U your-db-user -d gurls

# Применение миграций
\i migrate.sql
```

**Переменные окружения для продакшена:**

```env
DATABASE_PASSWORD=ваш_реальный_пароль
ENV=production
CONFIG_PATH=config/production.yml
DATABASE_AUTO_MIGRATE=false
DATABASE_SEED_DATA=false
```

### Управление миграциями

**Применение миграций:**

```bash
# Все миграции сразу
psql -h host -U user -d database -f migrations/migrate.sql

# Конкретная миграция
psql -h host -U user -d database -f migrations/001_create_subscription_types.sql
```

**Откат миграций (только для разработки!):**

```bash
psql -h localhost -U postgres -d gurls_test -f migrations/rollback.sql
```

### Проверка состояния БД

**Список таблиц:**

```sql
\dt
```

**Проверка данных:**

```sql
-- Типы подписок
SELECT * FROM subscription_types;

-- Количество пользователей
SELECT COUNT(*) FROM users;

-- Количество ссылок
SELECT COUNT(*) FROM links;
```

**Полезные запросы:**

```sql
-- Статистика по типам подписок
SELECT 
    st.name,
    st.display_name,
    COUNT(u.id) as users_count
FROM subscription_types st
LEFT JOIN users u ON u.subscription_type_id = st.id
GROUP BY st.id, st.name, st.display_name;

-- Топ 10 самых популярных ссылок
SELECT 
    alias,
    original_url,
    click_count
FROM links 
WHERE is_active = true
ORDER BY click_count DESC 
LIMIT 10;

-- Аналитика кликов по устройствам
SELECT 
    device_type,
    COUNT(*) as clicks_count
FROM clicks 
GROUP BY device_type 
ORDER BY clicks_count DESC;
```

### JWT Авторизация

**Архитектура JWT:**

Система поддерживает два типа токенов:

1. **Access Token** (JWT) - короткоживущий (15-30 минут)
2. **Refresh Token** - долгоживущий (7-30 дней), хранится в БД

**Поля в таблице users для JWT:**

- `email_verification_token` - токен для подтверждения email
- `password_reset_token` - токен для сброса пароля
- `password_reset_expires_at` - срок действия токена сброса
- `last_login_at` - время последнего входа

**Таблица refresh_tokens:**

```sql
-- Пример создания refresh токена
INSERT INTO refresh_tokens (
    user_id, token, expires_at, user_agent, ip_address
) VALUES (
    1, 'random_token_string', NOW() + INTERVAL '30 days', 
    'Mozilla/5.0...', '192.168.1.1'
);
```

**Управление токенами:**

```sql
-- Отзыв всех токенов пользователя
UPDATE refresh_tokens 
SET is_revoked = true 
WHERE user_id = 1 AND is_revoked = false;

-- Очистка истекших токенов
DELETE FROM refresh_tokens 
WHERE expires_at < NOW() OR is_revoked = true;

-- Активные токены пользователя
SELECT token, expires_at, last_used_at, user_agent 
FROM refresh_tokens 
WHERE user_id = 1 AND is_revoked = false AND expires_at > NOW();
```

### Безопасность

1. **Пароли**: Никогда не храните пароли в git репозитории
2. **SSL**: Используйте SSL соединения в продакшне
3. **JWT**: Храните секретные ключи в переменных окружения
4. **Refresh токены**: Регулярно очищайте истекшие токены
5. **Бэкапы**: Регулярно создавайте бэкапы базы данных
6. **Права доступа**: Предоставляйте минимальные необходимые права

### Мониторинг

**Размер базы данных:**

```sql
SELECT pg_size_pretty(pg_database_size('gurls')) as db_size;
```

**Размер таблиц:**

```sql
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables 
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

**Активные соединения:**

```sql
SELECT count(*) FROM pg_stat_activity WHERE datname = 'gurls';
```