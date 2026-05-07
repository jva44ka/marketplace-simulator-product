package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

// Transactor begins a DB transaction and hands the live pgx.Tx to the caller.
// Inside fn, call repo.WithTx(tx) to bind repositories to the transaction.
type Transactor interface {
	InTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// ProductTxRepository contains the product writes that must run inside a transaction.
type ProductTxRepository interface {
	Update(ctx context.Context, products []*models.Product) error
}

// ProductRepository is the full interface for product persistence.
type ProductRepository interface {
	GetBySku(ctx context.Context, sku uint64, txId *uint32) (*models.Product, error)
	GetBySkus(ctx context.Context, skus []uint64) ([]*models.Product, error)
	WithTx(tx pgx.Tx) ProductTxRepository
}

// ReservationTxRepository contains the reservation writes that must run inside a transaction.
type ReservationTxRepository interface {
	Insert(ctx context.Context, sku uint64, count uint32) (models.Reservation, error)
	DeleteByIds(ctx context.Context, ids []int64) error
}

// ReservationRepository is the full interface for reservation persistence.
type ReservationRepository interface {
	GetByIds(ctx context.Context, ids []int64) ([]models.Reservation, error)
	GetExpired(ctx context.Context, cutoff time.Time) ([]models.Reservation, error)
	WithTx(tx pgx.Tx) ReservationTxRepository
}

// ProductEventsOutboxTxRepository contains the outbox write that must run inside a transaction.
type ProductEventsOutboxTxRepository interface {
	Create(ctx context.Context, record models.ProductEventOutboxRecordNew) error
}

// ProductEventsOutboxRepository is the full interface for product-event outbox persistence.
type ProductEventsOutboxRepository interface {
	GetPending(ctx context.Context, limit int) ([]models.ProductEventOutboxRecord, error)
	GetCount(ctx context.Context, isDeadLetter bool) (int64, error)
	DeleteBatch(ctx context.Context, recordIds []uuid.UUID) error
	IncrementRetry(ctx context.Context, recordId uuid.UUID) error
	MarkDeadLetter(ctx context.Context, recordId uuid.UUID, reason string) error
	WithTx(tx pgx.Tx) ProductEventsOutboxTxRepository
}

// CacheUpdateOutboxTxRepository contains the outbox write that must run inside a transaction.
type CacheUpdateOutboxTxRepository interface {
	Create(ctx context.Context, sku uint64) error
}

// CacheUpdateOutboxRepository is the full interface for cache-update outbox persistence.
type CacheUpdateOutboxRepository interface {
	GetPending(ctx context.Context, limit int) ([]models.CacheUpdateOutboxRecord, error)
	CountPending(ctx context.Context) (int64, error)
	CountDeadLetters(ctx context.Context) (int64, error)
	DeleteBatch(ctx context.Context, ids []uuid.UUID) error
	IncrementRetry(ctx context.Context, id uuid.UUID) error
	MarkDeadLetter(ctx context.Context, id uuid.UUID, reason string) error
	WithTx(tx pgx.Tx) CacheUpdateOutboxTxRepository
}
