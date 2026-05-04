# marketplace-simulator-product

Микросервис управления товарами в рамках учебного проекта «Симулятор маркетплейса».

## Стек

- **Go** — язык реализации
- **gRPC** + **grpc-gateway** — транспортный слой (gRPC + REST HTTP-обёртка)
- **PostgreSQL** — хранилище товаров и резервирований (pgx/v5, pgxpool)
- **Kafka** — публикация событий об изменении товаров (segmentio/kafka-go)
- **etcd** — хранилище динамической конфигурации (hot-reload без рестарта)
- **goose** — миграции БД
- **Prometheus** — метрики
- **OpenTelemetry** — распределённые трейсы (OTLP → Tempo)
- **Swagger UI** — документация API

## Архитектура

```
cmd/server/          — точка входа
internal/
  app/               — gRPC-сервер, HTTP-сервер (grpc-gateway), middleware
  models/            — доменные модели (Product, Reservation)
  errors/            — доменные ошибки
  services/          — бизнес-логика (product, reservation)
  jobs/
    product_events_outbox  — публикация событий товаров в Kafka
    reservation_expiry     — снятие просроченных резервирований
    outbox_monitor         — сбор метрик outbox и пула соединений
  infra/
    config/          — загрузка конфигурации из YAML + ConfigStore (atomic hot-reload)
    etcd/            — etcd-клиент, чтение/seed конфига, watcher
    database/        — пул соединений, репозитории
    kafka/           — Kafka-продюсер событий товаров
    metrics/         — Prometheus-метрики
    tracing/         — инициализация OpenTelemetry
api/v1/              — Protobuf-определения
migrations/          — SQL-миграции (goose)
swagger/             — сгенерированный OpenAPI-файл
```

## API

### gRPC (порт 8002)

| Метод                  | Описание                                                                          |
|------------------------|-----------------------------------------------------------------------------------|
| `GetProduct`           | Получить информацию о товаре по SKU                                               |
| `IncreaseProductCount` | Увеличить количество товаров на складе                                            |
| `ReserveProduct`       | Зарезервировать товары (создаёт запись резервирования, остатки не изменяются)     |
| `ReleaseReservation`   | Снять резервирование (удаляет запись, остатки не изменяются) — **идемпотентен**   |
| `ConfirmReservation`   | Подтвердить покупку (списывает товар со склада, удаляет резервирование) — **идемпотентен** |

> **Идемпотентность `ReleaseReservation` и `ConfirmReservation`**: если резервирования по переданным ID уже не существуют (удалены в предыдущем вызове), методы возвращают успех без повторного изменения остатков. Это гарантирует корректность при at-least-once доставке из outbox.

### HTTP REST (порт 5001, grpc-gateway)

| Метод  | Путь                                   | Описание                              |
|--------|----------------------------------------|---------------------------------------|
| GET    | `/v1/products/{sku}`                   | Получить товар по SKU                 |
| POST   | `/v1/products/increase-count`          | Увеличить количество товаров          |
| POST   | `/v1/products/reserve`                 | Зарезервировать товар                 |
| POST   | `/v1/products/release-reservation`     | Снять резервирование                  |
| POST   | `/v1/products/confirm-reservation`     | Подтвердить покупку                   |
| GET    | `/metrics`                             | Prometheus-метрики                    |
| GET    | `/swagger/`                            | Swagger UI                            |
| GET    | `/api/`                                | OpenAPI JSON                          |

### Авторизация

Все запросы требуют заголовка `X-Auth`. Значение должно совпадать с `authorization.admin-user` из конфига.  
Отключить: `authorization.enabled: false`.

### Пример

```
GET http://localhost:5001/v1/products/1
X-Auth: admin
```
```json
{"sku": 1, "name": "Крем для лица", "count": 10, "price": 100.0}
```

## Конфигурация

Путь до файла задаётся переменной окружения `CONFIG_PATH`.

```yaml
http-server:
  host:
  port: 5000

grpc-server:
  host: localhost
  port: 8002

database:
  user: product
  password: product
  host: product-db
  port: 5432
  name: marketplace-simulator-product

authorization:
  enabled: true
  admin-user: admin

logging:
  log-request-body: true
  log-response-body: true

kafka:
  brokers:
    - kafka:9092
  product-events-topic: product.events
  write-timeout: 5s

rate-limiter:
  enabled: true
  rps: 500
  burst: 50

tracing:
  enabled: true
  otlp-endpoint: tempo:4317

etcd:
  endpoints:
    - etcd:2379
  dial-timeout: 5s
  config-key: /config/product   # ключ в etcd где хранится конфиг

jobs:
  reservation-expiry:
    enabled: true
    ttl: 5m
    job-interval: 1s
  product-events-outbox:
    enabled: true
    idle-interval: 10ms   # пауза когда очередь пуста
    active-interval: 0s   # пауза когда в прошлом тике были записи (0 = сразу)
    batch-size: 100
    max-retries: 5
  product-events-outbox-monitor:
    enabled: true
    job-interval: 10s
```

## Динамическая конфигурация (etcd)

При старте сервис читает конфиг из YAML, затем подключается к etcd:
- если ключ существует — загружает конфиг из etcd поверх YAML-дефолтов;
- если ключа нет — записывает YAML-конфиг в etcd (первый старт).

Затем запускает `Watch` на ключ — любое изменение в etcd применяется в реальном времени.

Если etcd недоступен при старте — сервис продолжает работу с YAML-конфигом (graceful degradation).

| Параметр | Обновляется без рестарта | Механизм |
|---|---|---|
| `rate-limiter.rps` / `burst` / `enabled` | ✅ | `limiter.SetLimit/SetBurst` в callback |
| `authorization.enabled` / `admin-user` | ✅ | middleware читает `cfgStore.Load()` на каждом запросе |
| `logging.log-request-body` / `log-response-body` | ✅ | middleware читает `cfgStore.Load()` на каждом запросе |
| `jobs.*.enabled` / `job-interval` / `batch-size` / `max-retries` / `ttl` | ✅ | джобы читают `cfgStore.Load()` на каждом тике |
| `database.*` | ⚠️ требует рестарта | лог warning при изменении |
| `http-server.*` / `grpc-server.*` | ⚠️ требует рестарта | лог warning при изменении |
| `kafka.brokers` / `topic` | ⚠️ требует рестарта | лог warning при изменении |
| `tracing.*` | ⚠️ требует рестарта | лог warning при изменении |

### Изменить конфиг через etcdctl

```bash
# Посмотреть текущий конфиг
docker exec etcd etcdctl get /config/product

# Изменить rate limiter
docker exec etcd etcdctl put /config/product "$(
  docker exec etcd etcdctl get /config/product --print-value-only \
  | sed 's/rps: 500/rps: 100/'
)"
```

Или через **etcd UI** → [http://localhost:8091](http://localhost:8091).

## Метрики Prometheus

| Метрика | Тип | Описание |
|---------|-----|----------|
| `requests_total{service,method,code}` | Counter | gRPC-запросы по методу и статус-коду |
| `request_duration_seconds{service,method}` | Histogram | Время обработки gRPC-запроса |
| `db_requests_total{service,method,status}` | Counter | Запросы к БД |
| `db_request_duration_seconds{service,method}` | Histogram | Длительность запросов к БД |
| `db_pool_acquired_conns{service}` | Gauge | Занятые соединения пула |
| `db_pool_idle_conns{service}` | Gauge | Свободные соединения пула |
| `db_pool_total_conns{service}` | Gauge | Всего соединений в пуле |
| `db_pool_max_conns{service}` | Gauge | Максимум соединений (MaxConns) |
| `db_pool_avg_acquire_duration_seconds{service}` | Gauge | Среднее время ожидания соединения |
| `db_optimistic_lock_failures_total{service}` | Counter | Сбои оптимистичной блокировки при обновлении остатков |
| `outbox_records_pending{service}` | Gauge | Записи outbox в очереди |
| `outbox_records_dead_letter{service}` | Gauge | Записи outbox в dead letter |
| `outbox_records_processed_total{service,status}` | Counter | Обработанные outbox-записи |

## Запуск локально

### Зависимости

- Go 1.25+
- PostgreSQL
- Kafka
- [goose](https://github.com/pressly/goose)

### Миграции

```bash
make up-migrations
```

### Сервер

```bash
CONFIG_PATH=configs/values_local.yaml go run ./cmd/server/main.go
```

## Docker

```bash
make docker-build-latest   # образ сервиса
make docker-build-migrator # образ мигратора
make docker-push-latest
make docker-push-migrator
```

## Генерация кода из proto

```bash
make proto-generate
```

Требует: `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`, `protoc-gen-grpc-gateway`, `protoc-gen-openapiv2`.
