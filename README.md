# marketplace-simulator-product

🇷🇺 Русский · [🇬🇧 English](README.en.md)

Микросервис управления товарами — часть учебного проекта «Симулятор маркетплейса».

gRPC + REST (grpc-gateway) на Go, PostgreSQL, Redis (кеш чтений), Kafka Outbox для публикации событий об изменении товаров. Rate limiter, Prometheus-метрики, OpenTelemetry-трейсы.

→ [Подробная документация](docs/README.md)

Запуск в составе полного стека: [marketplace-simulator](https://github.com/jva44ka/marketplace-simulator)
