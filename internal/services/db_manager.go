package services

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
)

type ProductReadRepository interface {
	GetBySku(ctx context.Context, sku uint64) (*models.Product, error)
	GetBySkus(ctx context.Context, skus []uint64) ([]*models.Product, error)
	WithTx(tx pgx.Tx) ProductWriteRepository
}

type ProductWriteRepository interface {
	Update(ctx context.Context, products []*models.Product) error
}

type ReservationReadRepository interface {
	GetByIds(ctx context.Context, ids []int64) ([]models.Reservation, error)
	WithTx(tx pgx.Tx) ReservationWriteRepository
}

type ReservationWriteRepository interface {
	Insert(ctx context.Context, sku uint64, count uint32) (models.Reservation, error)
	DeleteByIds(ctx context.Context, ids []int64) error
}

type ProductEventsOutboxReadRepository interface {
	GetPending(ctx context.Context, limit int) ([]models.ProductEventOutboxRecord, error)
	WithTx(tx pgx.Tx) ProductEventsOutboxWriteRepository
}

type ProductEventsOutboxWriteRepository interface {
	Create(ctx context.Context, record models.ProductEventOutboxRecordNew) error
	Delete(ctx context.Context, recordId string) error
	DeleteBatch(ctx context.Context, recordIds []string) error
	IncrementRetry(ctx context.Context, recordId string) error
	IncrementRetryBatch(ctx context.Context, recordIds []string) error
	MarkDeadLetter(ctx context.Context, recordId string, reason string) error
	MarkDeadLetterBatch(ctx context.Context, recordIds []string, reason string) error
}

type DBManager interface {
	ProductsRepo() ProductReadRepository
	ReservationsRepo() ReservationReadRepository
	ProductEventsOutboxRepo() ProductEventsOutboxReadRepository
	InTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}
