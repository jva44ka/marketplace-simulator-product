package reservation

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/postgres"
)

type Metrics interface {
	ReportRequest(method, status string)
}

type PgxRepository struct {
	pool    *pgxpool.Pool
	metrics Metrics
}

func NewPgxRepository(pool *pgxpool.Pool, metrics Metrics) *PgxRepository {
	return &PgxRepository{pool: pool, metrics: metrics}
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r *PgxRepository) querier(ctx context.Context) querier {
	if tx, ok := postgres.TxFromContext(ctx); ok {
		return tx
	}
	return r.pool
}

func (r *PgxRepository) Insert(ctx context.Context, sku uint64, count uint32) (Reservation, error) {
	const query = `
INSERT INTO reservations (sku, count)
VALUES ($1, $2)
RETURNING id, sku, count, created_at`

	var rv Reservation
	var skuInt int64
	var countInt int32
	err := r.querier(ctx).QueryRow(ctx, query, int64(sku), int32(count)).Scan(&rv.Id, &skuInt, &countInt, &rv.CreatedAt)
	if err != nil {
		r.metrics.ReportRequest("InsertReservation", "error")
		return Reservation{}, fmt.Errorf("PgxRepository.Insert: %w", err)
	}
	rv.Sku = uint64(skuInt)
	rv.Count = uint32(countInt)

	r.metrics.ReportRequest("InsertReservation", "success")
	return rv, nil
}

func (r *PgxRepository) GetByIds(ctx context.Context, ids []int64) ([]Reservation, error) {
	const query = `
SELECT id, sku, count, created_at
FROM reservations
WHERE id = ANY($1)`

	rows, err := r.querier(ctx).Query(ctx, query, ids)
	if err != nil {
		r.metrics.ReportRequest("GetReservationsByIds", "error")
		return nil, fmt.Errorf("PgxRepository.GetByIds: %w", err)
	}
	defer rows.Close()

	var result []Reservation
	for rows.Next() {
		var rv Reservation
		var sku int64
		var count int32
		if err = rows.Scan(&rv.Id, &sku, &count, &rv.CreatedAt); err != nil {
			r.metrics.ReportRequest("GetReservationsByIds", "error")
			return nil, fmt.Errorf("PgxRepository.GetByIds: %w", err)
		}
		rv.Sku = uint64(sku)
		rv.Count = uint32(count)
		result = append(result, rv)
	}

	r.metrics.ReportRequest("GetReservationsByIds", "success")
	return result, nil
}

func (r *PgxRepository) DeleteByIds(ctx context.Context, ids []int64) error {
	const query = `DELETE FROM reservations WHERE id = ANY($1)`

	_, err := r.querier(ctx).Exec(ctx, query, ids)
	if err != nil {
		r.metrics.ReportRequest("DeleteReservationsByIds", "error")
		return fmt.Errorf("PgxRepository.DeleteByIds: %w", err)
	}

	r.metrics.ReportRequest("DeleteReservationsByIds", "success")
	return nil
}

func (r *PgxRepository) GetExpired(ctx context.Context, cutoff time.Time) ([]Reservation, error) {
	const query = `
SELECT id, sku, count, created_at
FROM reservations
WHERE created_at < $1`

	rows, err := r.pool.Query(ctx, query, cutoff)
	if err != nil {
		r.metrics.ReportRequest("GetExpiredReservations", "error")
		return nil, fmt.Errorf("PgxRepository.GetExpired: %w", err)
	}
	defer rows.Close()

	var result []Reservation
	for rows.Next() {
		var rv Reservation
		var sku int64
		var count int32
		if err = rows.Scan(&rv.Id, &sku, &count, &rv.CreatedAt); err != nil {
			r.metrics.ReportRequest("GetExpiredReservations", "error")
			return nil, fmt.Errorf("PgxRepository.GetExpired: %w", err)
		}
		rv.Sku = uint64(sku)
		rv.Count = uint32(count)
		result = append(result, rv)
	}

	r.metrics.ReportRequest("GetExpiredReservations", "success")
	return result, nil
}
