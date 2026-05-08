package transactor

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/database/repository"
	svcReservation "github.com/jva44ka/marketplace-simulator-product/internal/services/reservation"
)

type ReservationServiceTransactor struct {
	pool               *pgxpool.Pool
	productMetrics     repository.ProductRepositoryMetrics
	reservationMetrics repository.ReservationRepositoryMetrics
}

func NewReservationServiceTransactor(
	pool *pgxpool.Pool,
	productMetrics repository.ProductRepositoryMetrics,
	reservationMetrics repository.ReservationRepositoryMetrics,
) *ReservationServiceTransactor {
	return &ReservationServiceTransactor{
		pool:               pool,
		productMetrics:     productMetrics,
		reservationMetrics: reservationMetrics,
	}
}

func (t *ReservationServiceTransactor) InTransaction(
	ctx context.Context,
	fn func(
		products      svcReservation.TxProductRepository,
		reservations  svcReservation.TxReservationRepository,
		productEvents svcReservation.TxProductEventsOutboxRepository,
		cacheUpdates  svcReservation.TxCacheUpdateOutboxRepository,
	) error,
) error {
	return pgx.BeginTxFunc(ctx, t.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return fn(
			repository.NewProductPgxTxRepository(tx, t.productMetrics),
			repository.NewReservationPgxTxRepository(tx, t.reservationMetrics),
			repository.NewProductEventsOutboxTxRepository(tx),
			repository.NewCacheUpdateOutboxTxRepository(tx),
		)
	})
}
