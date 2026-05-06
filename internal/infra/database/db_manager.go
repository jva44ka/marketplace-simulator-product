package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/database/repositories"
	"github.com/jva44ka/marketplace-simulator-product/internal/services"
)

type DBManager struct {
	pool         *pgxpool.Pool
	products     services.ProductRepository // may be wrapped by CachedProductRepository
	reservations *repositories.ReservationPgxRepository
	outbox       *repositories.ProductEventsOutboxPgxRepository
	cacheOutbox  *repositories.CacheUpdateOutboxPgxRepository
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
		cacheOutbox:  repositories.NewCacheUpdateOutboxRepository(pool),
	}
}

// TODO: убрать после разбиение этого менеджера на transactor / repositories
func (m *DBManager) SetProductsRepo(r services.ProductRepository) {
	m.products = r
}

func (m *DBManager) ProductsRepo() services.ProductRepository {
	return m.products
}

func (m *DBManager) ReservationsRepo() services.ReservationRepository {
	return m.reservations
}

func (m *DBManager) ReservationPgxRepo() *repositories.ReservationPgxRepository {
	return m.reservations
}

func (m *DBManager) ProductEventsOutboxRepo() services.ProductEventsOutboxRepository {
	return m.outbox
}

func (m *DBManager) CacheUpdateOutboxRepo() services.CacheUpdateOutboxRepository {
	return m.cacheOutbox
}

func (m *DBManager) InTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, m.pool, pgx.TxOptions{}, fn)
}
