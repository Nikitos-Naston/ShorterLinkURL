# LinkShrink

Простой HTTP-сервис для сокращения ссылок на Go.

Сервис генерирует уникальный код длиной 10 символов и поддерживает два типа хранилища:

- in-memory;
- PostgreSQL.

## Запуск

### Memory

```bash
go run ./cmd/linkShrinks -storage memory
```

Сервис будет доступен на:

```text
http://localhost:8080
```

### PostgreSQL через Docker Compose

```bash
docker compose up --build
```

Docker Compose автоматически:

1. запустит PostgreSQL;
2. выполнит миграции;
3. запустит LinkShrink с PostgreSQL storage.

Сервис будет доступен на:

```text
http://localhost:8080
```

## Конфигурация

Для PostgreSQL используется переменная окружения:

```text
DATABASE_URL
```

Например:

```text
postgres://postgres:postgres@localhost:5432/linkshrink?sslmode=disable
```

Опционально можно указать публичный адрес сервиса:

```text
BASE_URL=http://localhost:8080
```

## API

### Создать короткую ссылку

```http
POST /
Content-Type: application/json
```

Пример:

```json
{
  "url": "https://example.com/page"
}
```

Ответ:

```json
{
  "short_url": "http://localhost:8080/aB12_cdEF3"
}
```

### Получить оригинальный URL

```http
GET /aB12_cdEF3
```

Ответ:

```json
{
  "url": "https://example.com/page"
}
```

## Тесты

```bash
go test ./...
```

Проверка race conditions:

```bash
go test -race ./...
```