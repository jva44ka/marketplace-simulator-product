package reservation

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Reservation struct {
	Id        int64
	Sku       uint64
	Count     uint32
	CreatedAt time.Time
}

type Metrics interface {
	ReportRequest(method, status string)
}

type PgxReservationRepository struct {
	pool    *pgxpool.Pool
	metrics Metrics
}

func NewPgxReservationRepository(pool *pgxpool.Pool, metrics Metrics) *PgxReservationRepository {
	return &PgxReservationRepository{pool: pool, metrics: metrics}
}

func (r *PgxReservationRepository) Insert(ctx context.Context, sku uint64, count uint32) (int64, error) {
	const query = `
INSERT INTO reservations (sku, count)
VALUES ($1, $2)
RETURNING id`

	var id int64
	err := r.pool.QueryRow(ctx, query, int64(sku), int32(count)).Scan(&id)
	if err != nil {
		r.metrics.ReportRequest("InsertReservation", "error")
		return 0, fmt.Errorf("PgxReservationRepository.Insert: %w", err)
	}

	r.metrics.ReportRequest("InsertReservation", "success")
	return id, nil
}

func (r *PgxReservationRepository) GetByIds(ctx context.Context, ids []int64) ([]Reservation, error) {
	const query = `
SELECT id, sku, count, created_at
FROM reservations
WHERE id = ANY($1)`

	rows, err := r.pool.Query(ctx, query, ids)
	if err != nil {
		r.metrics.ReportRequest("GetReservationsByIds", "error")
		return nil, fmt.Errorf("PgxReservationRepository.GetByIds: %w", err)
	}
	defer rows.Close()

	var result []Reservation
	for rows.Next() {
		var rv Reservation
		var sku int64
		var count int32
		if err = rows.Scan(&rv.Id, &sku, &count, &rv.CreatedAt); err != nil {
			r.metrics.ReportRequest("GetReservationsByIds", "error")
			return nil, fmt.Errorf("PgxReservationRepository.GetByIds: %w", err)
		}
		rv.Sku = uint64(sku)
		rv.Count = uint32(count)
		result = append(result, rv)
	}

	r.metrics.ReportRequest("GetReservationsByIds", "success")
	return result, nil
}

func (r *PgxReservationRepository) DeleteByIds(ctx context.Context, ids []int64) error {
	const query = `DELETE FROM reservations WHERE id = ANY($1)`

	_, err := r.pool.Exec(ctx, query, ids)
	if err != nil {
		r.metrics.ReportRequest("DeleteReservationsByIds", "error")
		return fmt.Errorf("PgxReservationRepository.DeleteByIds: %w", err)
	}

	r.metrics.ReportRequest("DeleteReservationsByIds", "success")
	return nil
}

func (r *PgxReservationRepository) GetExpired(ctx context.Context, cutoff time.Time) ([]Reservation, error) {
	const query = `
SELECT id, sku, count, created_at
FROM reservations
WHERE created_at < $1`

	rows, err := r.pool.Query(ctx, query, cutoff)
	if err != nil {
		r.metrics.ReportRequest("GetExpiredReservations", "error")
		return nil, fmt.Errorf("PgxReservationRepository.GetExpired: %w", err)
	}
	defer rows.Close()

	var result []Reservation
	for rows.Next() {
		var rv Reservation
		var sku int64
		var count int32
		if err = rows.Scan(&rv.Id, &sku, &count, &rv.CreatedAt); err != nil {
			r.metrics.ReportRequest("GetExpiredReservations", "error")
			return nil, fmt.Errorf("PgxReservationRepository.GetExpired: %w", err)
		}
		rv.Sku = uint64(sku)
		rv.Count = uint32(count)
		result = append(result, rv)
	}

	r.metrics.ReportRequest("GetExpiredReservations", "success")
	return result, nil
}
