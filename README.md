# marketplace-simulator-product

Микросервис управления товарами — сервис в рамках учебного проекта «Симулятор маркетплейса».

## Стек

- **Go** — язык реализации
- **gRPC** + **grpc-gateway** — транспортный слой (gRPC + REST HTTP-обёртка)
- **PostgreSQL** — хранилище товаров и резервирований
- **Kafka** — публикация событий (истечение резервирований)
- **goose** — миграции БД
- **Prometheus** — сбор метрик
- **Swagger UI** — документация API

## Архитектура

```
cmd/server/          — точка входа
internal/
  app/               — gRPC-сервер, HTTP-сервер (grpc-gateway), middleware
  models/            — доменные модели (Product, Reservation)
  errors/            — доменные ошибки
  services/          — бизнес-логика
  jobs/              — фоновые задачи (авто-снятие просроченных резервирований)
  infra/
    config/          — загрузка конфигурации из YAML
    database/        — пул соединений, репозитории
    kafka/           — Kafka-продюсер
    metrics/         — Prometheus-метрики (запросы, БД, optimistic lock)
api/v1/              — Protobuf-определения
migrations/          — SQL-миграции (goose)
swagger/             — сгенерированный OpenAPI-файл
```

## API

### gRPC (порт 8082 по умолчанию)

| Метод                  | Описание                                           |
|------------------------|----------------------------------------------------|
| `GetProduct`           | Получить информацию о товаре по SKU                |
| `IncreaseProductCount` | Увеличить количество товаров на складе             |
| `ReserveProduct`       | Зарезервировать товар (создаёт запись резервации, остатки не изменяются) |
| `ReleaseReservation`   | Снять резервацию (удаляет запись резервации, остатки не изменяются)      |
| `ConfirmReservation`   | Подтвердить покупку (списывает товар со склада и удаляет резервацию)     |

### HTTP REST (порт 8080 по умолчанию, grpc-gateway)

| Метод  | Путь                                   | Описание                              |
|--------|----------------------------------------|---------------------------------------|
| GET    | `/v1/products/{sku}`                   | Получить товар по SKU                 |
| POST   | `/v1/products/increase-count`          | Увеличить количество товаров          |
| POST   | `/v1/products/reserve`                 | Зарезервировать товар (создаёт запись резервации)              |
| POST   | `/v1/products/release-reservation`     | Снять резервацию (удаляет запись резервации)                   |
| POST   | `/v1/products/confirm-reservation`     | Подтвердить покупку (списывает со склада, удаляет резервацию)  |
| GET    | `/metrics`                             | Prometheus-метрики                    |
| GET    | `/swagger/`                            | Swagger UI                            |
| GET    | `/api/`                                | OpenAPI JSON                          |

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
  name: marketplace_simulator_product

authorization:
  enabled: false
  admin-user: admin

logging:
  log-request-body: true
  log-response-body: true

kafka:
  brokers:
    - localhost:9092
  reservation-expired-topic: reservation.expired

reservation:
  ttl: 30m
  job-interval: 10m
```

## Запуск локально

### Зависимости

- Go 1.24+
- PostgreSQL
- Kafka
- [goose](https://github.com/pressly/goose)

### Миграции

```bash
make up-migrations
```

> По умолчанию подключается к `postgresql://postgres:1234@127.0.0.1:5432/marketplace_simulator_product`

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
| `products_grpc_requests_total`               | Counter   | Общее количество gRPC-запросов (method, code)                      |
| `products_grpc_request_duration_seconds`     | Histogram | Время обработки gRPC-запроса (method)                              |
| `products_db_requests_total`                 | Counter   | Запросы к БД (method, status)                                      |
| `products_db_optimistic_lock_failures_total` | Counter   | Сбои оптимистичной блокировки при обновлении остатков товаров      |

Доступны по адресу `GET /metrics`.

## Генерация кода из proto

```bash
make proto-generate
```

Требует установленных `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`, `protoc-gen-grpc-gateway`, `protoc-gen-openapiv2`.
