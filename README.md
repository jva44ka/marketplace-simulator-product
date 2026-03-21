# ozon-simulator-go-products

Микросервис управления товарами — учебный проект в рамках курса Route 256 «Продвинутая разработка микросервисов на Go».

## Стек

- **Go** — язык реализации
- **gRPC** + **grpc-gateway** — транспортный слой (gRPC + REST HTTP-обёртка)
- **PostgreSQL** — хранилище товаров
- **goose** — миграции БД
- **Prometheus** — сбор метрик
- **Swagger UI** — документация API

## Архитектура

```
cmd/server/          — точка входа
internal/
  app/               — gRPC-сервер, HTTP-сервер (grpc-gateway), middleware
  domain/
    model/           — доменные модели и ошибки
    repository/      — работа с PostgreSQL
    service/         — бизнес-логика
  infra/
    config/          — загрузка конфигурации из YAML
    metrics/         — Prometheus-метрики (запросы, БД, optimistic lock)
api/v1/              — Protobuf-определения
migrations/          — SQL-миграции (goose)
swagger/             — сгенерированный OpenAPI-файл
```

## API

### gRPC (порт 8082 по умолчанию)

| Метод                  | Описание                              |
|------------------------|---------------------------------------|
| `GetProduct`           | Получить информацию о товаре по SKU   |
| `IncreaseProductCount` | Увеличить количество товаров на складе|
| `DecreaseProductCount` | Уменьшить количество товаров на складе|

### HTTP REST (порт 8080 по умолчанию, grpc-gateway)

| Метод  | Путь                             | Описание                              |
|--------|----------------------------------|---------------------------------------|
| GET    | `/v1/products/{sku}`             | Получить товар по SKU                 |
| POST   | `/v1/products/increase-count`    | Увеличить количество товаров          |
| POST   | `/v1/products/decrease-count`    | Уменьшить количество товаров          |
| GET    | `/metrics`                       | Prometheus-метрики                    |
| GET    | `/swagger/`                      | Swagger UI                            |
| GET    | `/api/`                          | OpenAPI JSON                          |

### Пример запроса

```
GET http://localhost:8080/v1/products/1
X-Auth: admin
```

```json
{
  "sku": 1,
  "name": "Крем для лица",
  "count": 10,
  "price": 100.0
}
```

### Авторизация

Все запросы требуют заголовка `X-Auth`. Значение должно совпадать с `authorization.admin-user` из конфига.
Авторизацию можно отключить: `authorization.enabled: false`.

## Конфигурация

Путь до файла конфигурации задаётся переменной окружения `CONFIG_PATH`.

```yaml
http-server:
  host:
  port: 8080

grpc-server:
  host: localhost
  port: 8082

database:
  user: postgres
  password: 1234
  host: localhost
  port: 5432
  name: ozon_simulator_go_products

authorization:
  enabled: false
  admin-user: admin
```

## Запуск локально

### Зависимости

- Go 1.22+
- PostgreSQL
- [goose](https://github.com/pressly/goose)

### Миграции

```bash
make up-migrations
```

> По умолчанию подключается к `postgresql://postgres:1234@127.0.0.1:5432/ozon_simulator_go_products`

### Сервер

```bash
CONFIG_PATH=configs/values_local.yaml go run ./cmd/server/main.go
```

## Docker

```bash
# Собрать образ сервиса
make docker-build-latest

# Собрать образ мигратора
make docker-build-migrator

# Опубликовать
make docker-push-latest
make docker-push-migrator
```

## Метрики Prometheus

| Метрика                                      | Тип       | Описание                                     |
|----------------------------------------------|-----------|----------------------------------------------|
| `products_grpc_requests_total`               | Counter   | Общее количество gRPC-запросов (method, code)|
| `products_grpc_request_duration_seconds`     | Histogram | Время обработки gRPC-запроса (method)        |
| `products_db_requests_total`                 | Counter   | Запросы к БД (method, status)                |
| `products_db_optimistic_lock_failures_total` | Counter   | Количество сбоев оптимистичной блокировки    |

Доступны по адресу `GET /metrics`.

## Генерация кода из proto

```bash
make proto-generate
```

Требует установленных `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`, `protoc-gen-grpc-gateway`, `protoc-gen-openapiv2`.
