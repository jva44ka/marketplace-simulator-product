package data

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/product"
)

type Transactor struct {
	pool               *pgxpool.Pool
	productMetrics     RepositoryMetrics
	reservationMetrics ReservationMetrics
}

type Repositories struct {
	Products     product.ProductRepository
	Reservations product.ReservationRepository
}

func NewTransactor(pool *pgxpool.Pool, productMetrics RepositoryMetrics, reservationMetrics ReservationMetrics) *Transactor {
	return &Transactor{
		pool:               pool,
		productMetrics:     productMetrics,
		reservationMetrics: reservationMetrics,
	}
}

func (t *Transactor) InTransaction(ctx context.Context, fn func(repos Repositories) error) error {
	return pgx.BeginTxFunc(ctx, t.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		repos := Repositories{
			Products:     NewProductTxPgxRepository(tx, t.productMetrics),
			Reservations: NewReservationTxPgxRepository(tx, t.reservationMetrics),
		}
		return fn(repos)
	})
}
