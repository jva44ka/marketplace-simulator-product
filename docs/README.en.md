# marketplace-simulator-product

[🇷🇺 Русский](README.md) · 🇬🇧 English

Product management microservice in the "Marketplace Simulator" pet project.

## Stack

- **Go** — implementation language
- **gRPC** + **grpc-gateway** — transport layer (gRPC + REST HTTP wrapper)
- **PostgreSQL** — product and reservation storage (pgx/v5, pgxpool)
- **Redis** — read cache for products (go-redis/v9); Cache-Aside via repository decorator; graceful degradation when unavailable
- **Kafka** — publishing product change events (segmentio/kafka-go)
- **etcd** — dynamic configuration store (hot-reload without restarts)
- **goose** — database migrations
- **Prometheus** — metrics
- **OpenTelemetry** — distributed traces (OTLP → Tempo); instrumented: gRPC, PostgreSQL, Redis
- **Swagger UI** — API documentation

## Architecture

```
cmd/server/          — entry point
internal/
  app/               — gRPC server, HTTP server (grpc-gateway), middleware
  models/            — domain models (Product, Reservation)
  errors/            — domain errors
  usecases/
    product/         — product use cases
      get_product.go       — get product by SKU
      increase_count.go    — increase stock count
      transactor.go        — interfaces
    reservation/     — reservation use cases
      reserve.go           — create a reservation
      release.go           — release a reservation
      confirm.go           — confirm a purchase
      transactor.go        — interfaces
  services/
    outbox/          — build outbox records for product change events
  jobs/
    product_events_outbox  — publish product events to Kafka
    cache_update_outbox    — invalidate/update Redis after product changes
    reservation_expiry     — release expired reservations
    outbox_monitor         — collect outbox and connection pool metrics
  infra/
    config/          — YAML config loading + ConfigStore (atomic hot-reload)
    etcd/            — etcd client, config read/seed, watcher
    database/        — connection pool, repositories
    cache/           — Redis client (CacheClient[T]), CachedProductRepository decorator
    kafka/           — Kafka producer for product events
    metrics/         — Prometheus metrics
    tracing/         — OpenTelemetry initialisation
api/v1/              — Protobuf definitions
migrations/          — SQL migrations (goose)
swagger/             — generated OpenAPI file
```

## API

### gRPC (port 8002)

| Method                 | Description                                                                             |
|------------------------|-----------------------------------------------------------------------------------------|
| `GetProduct`           | Get product information by SKU                                                          |
| `IncreaseProductCount` | Increase stock count                                                                    |
| `ReserveProduct`       | Reserve products (creates a reservation record, stock is not changed yet)               |
| `ReleaseReservation`   | Release a reservation (deletes the record, stock unchanged) — **idempotent**            |
| `ConfirmReservation`   | Confirm a purchase (deducts stock, deletes the reservation) — **idempotent**            |

> **Idempotency of `ReleaseReservation` and `ConfirmReservation`**: if reservations with the given IDs no longer exist (deleted in a previous call), the methods return success without modifying stock again. This guarantees correctness under at-least-once delivery from the outbox.

### HTTP REST (port 5001, grpc-gateway)

| Method | Path                                   | Description                           |
|--------|----------------------------------------|---------------------------------------|
| GET    | `/v1/products/{sku}`                   | Get product by SKU                    |
| POST   | `/v1/products/increase-count`          | Increase stock count                  |
| POST   | `/v1/products/reserve`                 | Reserve a product                     |
| POST   | `/v1/products/release-reservation`     | Release a reservation                 |
| POST   | `/v1/products/confirm-reservation`     | Confirm a purchase                    |
| GET    | `/metrics`                             | Prometheus metrics                    |
| GET    | `/swagger/`                            | Swagger UI                            |
| GET    | `/api/`                                | OpenAPI JSON                          |

### Authorization

All requests require the `X-Auth` header. The value must match `authorization.admin-user` from the config.  
To disable: `authorization.enabled: false`.

### Example

```
GET http://localhost:5001/v1/products/1
X-Auth: admin
```
```json
{"sku": 1, "name": "Face cream", "count": 10, "price": 100.0}
```

## Configuration

The config file path is set via the `CONFIG_PATH` environment variable.

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

redis:
  enabled: true        # false → graceful degradation, all reads go to DB
  address: redis:6379
  ttl: 5m              # cache entry TTL; recommended not to exceed reservation lifetime

etcd:
  endpoints:
    - etcd:2379
  dial-timeout: 5s
  config-key: /config/product   # etcd key where the config is stored

jobs:
  reservation-expiry:
    enabled: true
    ttl: 5m
    job-interval: 1s
  product-events-outbox:
    enabled: true
    idle-interval: 10ms   # pause when queue is empty
    active-interval: 0s   # pause when previous tick had records (0 = immediately)
    batch-size: 100
    max-retries: 5
  product-events-outbox-monitor:
    enabled: true
    job-interval: 10s
  cache-update-outbox:
    enabled: true
    idle-interval: 100ms  # pause when no records
    active-interval: 0s   # pause when previous tick had records
    batch-size: 100
    max-retries: 5
```

## Dynamic configuration (etcd)

On startup the service reads config from YAML, then connects to etcd:
- if the key exists — loads config from etcd on top of YAML defaults;
- if the key is absent — writes the YAML config to etcd (first start).

It then starts a `Watch` on the key — any change in etcd is applied in real time.

If etcd is unavailable on startup — the service continues with YAML config (graceful degradation).

| Parameter | Updated without restart | Mechanism |
|---|---|---|
| `rate-limiter.rps` / `burst` / `enabled` | ✅ | `limiter.SetLimit/SetBurst` in callback |
| `authorization.enabled` / `admin-user` | ✅ | middleware reads `cfgStore.Load()` on every request |
| `logging.log-request-body` / `log-response-body` | ✅ | middleware reads `cfgStore.Load()` on every request |
| `jobs.*.enabled` / `job-interval` / `batch-size` / `max-retries` / `ttl` | ✅ | jobs read `cfgStore.Load()` on every tick |
| `database.*` | ⚠️ requires restart | warning log on change |
| `redis.*` | ⚠️ requires restart | warning log on change |
| `http-server.*` / `grpc-server.*` | ⚠️ requires restart | warning log on change |
| `kafka.brokers` / `topic` | ⚠️ requires restart | warning log on change |
| `tracing.*` | ⚠️ requires restart | warning log on change |

### Change config via etcdctl

```bash
# View current config
docker exec etcd etcdctl get /config/product

# Change rate limiter
docker exec etcd etcdctl put /config/product "$(
  docker exec etcd etcdctl get /config/product --print-value-only \
  | sed 's/rps: 500/rps: 100/'
)"
```

Or via **etcd UI** → [http://localhost:8091](http://localhost:8091).

## Prometheus metrics

| Metric | Type | Description |
|--------|------|-------------|
| `requests_total{service,method,code}` | Counter | gRPC requests by method and status code |
| `request_duration_seconds{service,method}` | Histogram | gRPC request processing time |
| `db_requests_total{service,method,status}` | Counter | Database requests |
| `db_request_duration_seconds{service,method}` | Histogram | Database request duration |
| `db_pool_acquired_conns{service}` | Gauge | Acquired pool connections |
| `db_pool_idle_conns{service}` | Gauge | Idle pool connections |
| `db_pool_total_conns{service}` | Gauge | Total pool connections |
| `db_pool_max_conns{service}` | Gauge | Maximum connections (MaxConns) |
| `db_pool_avg_acquire_duration_seconds{service}` | Gauge | Average connection wait time |
| `db_optimistic_lock_failures_total{service}` | Counter | Optimistic lock failures on stock updates |
| `outbox_records_pending{service,outbox}` | Gauge | Outbox records in queue (by outbox type) |
| `outbox_records_dead_letter{service,outbox}` | Gauge | Outbox records in dead letter (by outbox type) |
| `outbox_records_processed_total{service,job,status}` | Counter | Processed outbox records |
| `products_cache_operations_total{service,operation,result}` | Counter | Redis operations: operation=get/set/delete, result=hit/miss/stale/error/success |
| `products_cache_operation_duration_seconds{service,operation}` | Histogram | Redis operation duration by type |

## Running locally

### Dependencies

- Go 1.25+
- PostgreSQL
- Redis 7+
- Kafka
- [goose](https://github.com/pressly/goose)

### Migrations

```bash
make up-migrations
```

### Server

```bash
CONFIG_PATH=configs/values_local.yaml go run ./cmd/server/main.go
```

## Docker

```bash
make docker-build-latest   # service image
make docker-build-migrator # migrator image
make docker-push-latest
make docker-push-migrator
```

## Code generation from proto

```bash
make proto-generate
```

Requires: `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc`, `protoc-gen-grpc-gateway`, `protoc-gen-openapiv2`.
