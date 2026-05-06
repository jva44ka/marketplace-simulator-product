package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

type ProductRepository interface {
	GetBySku(ctx context.Context, sku uint64) (*models.Product, error)
	GetBySkus(ctx context.Context, skus []uint64) ([]*models.Product, error)
	WithTx(tx pgx.Tx) ProductTxRepository
}

type ProductTxRepository interface {
	Update(ctx context.Context, products []*models.Product) error
}

type ReservationRepository interface {
	GetByIds(ctx context.Context, ids []int64) ([]models.Reservation, error)
	WithTx(tx pgx.Tx) ReservationTxRepository
}

type ReservationTxRepository interface {
	Insert(ctx context.Context, sku uint64, count uint32) (models.Reservation, error)
	DeleteByIds(ctx context.Context, ids []int64) error
}

type ProductEventsOutboxRepository interface {
	GetPending(ctx context.Context, limit int) ([]models.ProductEventOutboxRecord, error)
	GetCount(ctx context.Context, isDeadLetter bool) (int64, error)
	DeleteBatch(ctx context.Context, recordIds []uuid.UUID) error
	IncrementRetry(ctx context.Context, recordId uuid.UUID) error
	MarkDeadLetter(ctx context.Context, recordId uuid.UUID, reason string) error
	WithTx(tx pgx.Tx) ProductEventsOutboxTxRepository
}

type ProductEventsOutboxTxRepository interface {
	Create(ctx context.Context, record models.ProductEventOutboxRecordNew) error
}

type CacheUpdateOutboxRepository interface {
	GetPending(ctx context.Context, limit int) ([]models.CacheUpdateOutboxRecord, error)
	CountPending(ctx context.Context) (int64, error)
	CountDeadLetters(ctx context.Context) (int64, error)
	DeleteBatch(ctx context.Context, ids []uuid.UUID) error
	IncrementRetry(ctx context.Context, id uuid.UUID) error
	MarkDeadLetter(ctx context.Context, id uuid.UUID, reason string) error
	WithTx(tx pgx.Tx) CacheUpdateOutboxTxRepository
}

type CacheUpdateOutboxTxRepository interface {
	Create(ctx context.Context, sku uint64) error
}

type DBManager interface {
	ProductsRepo() ProductRepository
	ReservationsRepo() ReservationRepository
	ProductEventsOutboxRepo() ProductEventsOutboxRepository
	CacheUpdateOutboxRepo() CacheUpdateOutboxRepository
	InTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}
