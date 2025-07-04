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