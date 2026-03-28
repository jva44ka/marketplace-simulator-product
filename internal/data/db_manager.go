package data

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain"
)

type DBManager struct {
	pool         *pgxpool.Pool
	products     *ProductPgxRepository
	reservations *ReservationPgxRepository
}

func NewDBManager(pool *pgxpool.Pool, productMetrics RepositoryMetrics, reservationMetrics ReservationMetrics) *DBManager {
	return &DBManager{
		pool:         pool,
		products:     NewProductPgxRepository(pool, productMetrics),
		reservations: NewReservationPgxRepository(pool, reservationMetrics),
	}
}

func (m *DBManager) Products() domain.ProductReadRepository {
	return m.products
}

func (m *DBManager) Reservations() domain.ReservationReadRepository {
	return m.reservations
}

func (m *DBManager) ReservationRepo() *ReservationPgxRepository {
	return m.reservations
}

func (m *DBManager) InTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, m.pool, pgx.TxOptions{}, fn)
}
