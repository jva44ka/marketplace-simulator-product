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

type DBManager interface {
	Products() ProductReadRepository
	Reservations() ReservationReadRepository
	InTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}
