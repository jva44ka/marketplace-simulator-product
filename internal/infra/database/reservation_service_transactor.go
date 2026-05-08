package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	svcReservation "github.com/jva44ka/marketplace-simulator-product/internal/services/reservation"
)

type ReservationServiceTransactor struct {
	pool                *pgxpool.Pool
	products            *ProductPgxRepository
	reservations        *ReservationPgxRepository
	productEventsOutbox *ProductEventsOutboxRepository
	cacheUpdateOutbox   *CacheUpdateOutboxRepository
}

func NewReservationServiceTransactor(
	pool *pgxpool.Pool,
	products *ProductPgxRepository,
	reservations *ReservationPgxRepository,
	productEventsOutbox *ProductEventsOutboxRepository,
	cacheUpdateOutbox *CacheUpdateOutboxRepository,
) *ReservationServiceTransactor {
	return &ReservationServiceTransactor{
		pool:                pool,
		products:            products,
		reservations:        reservations,
		productEventsOutbox: productEventsOutbox,
		cacheUpdateOutbox:   cacheUpdateOutbox,
	}
}

func (t *ReservationServiceTransactor) InTransaction(
	ctx context.Context,
	fn func(
		products svcReservation.TxProductRepository,
		reservations svcReservation.TxReservationRepository,
		productEvents svcReservation.TxProductEventsOutboxRepository,
		cacheUpdates svcReservation.TxCacheUpdateOutboxRepository,
	) error,
) error {
	return pgx.BeginTxFunc(ctx, t.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return fn(
			t.products.WithTx(tx),
			t.reservations.WithTx(tx),
			t.productEventsOutbox.WithTx(tx),
			t.cacheUpdateOutbox.WithTx(tx),
		)
	})
}
