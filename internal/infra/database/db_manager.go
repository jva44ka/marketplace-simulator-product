package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/database/repositories"
	"github.com/jva44ka/ozon-simulator-go-products/internal/services"
)

type DBManager struct {
	pool         *pgxpool.Pool
	products     *repositories.ProductPgxRepository
	reservations *repositories.ReservationPgxRepository
	outbox       *repositories.ProductEventsOutboxPgxRepository
}

func NewDBManager(
	pool *pgxpool.Pool,
	productMetrics repositories.RepositoryMetrics,
	reservationMetrics repositories.ReservationMetrics) *DBManager {
	return &DBManager{
		pool:         pool,
		products:     repositories.NewProductPgxRepository(pool, productMetrics),
		reservations: repositories.NewReservationPgxRepository(pool, reservationMetrics),
		outbox:       repositories.NewOutboxPgxRepository(pool),
	}
}

func (m *DBManager) ProductsRepo() services.ProductReadRepository {
	return m.products
}

func (m *DBManager) ReservationsRepo() services.ReservationReadRepository {
	return m.reservations
}

func (m *DBManager) ReservationPgxRepo() *repositories.ReservationPgxRepository {
	return m.reservations
}

func (m *DBManager) ProductEventsOutboxRepo() services.ProductEventsOutboxReadRepository {
	return m.outbox
}

func (m *DBManager) InTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, m.pool, pgx.TxOptions{}, fn)
}
