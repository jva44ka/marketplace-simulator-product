package reservation

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Reservation struct {
	Id            int64
	Sku           uint64
	Count         uint32
	ReservedUntil time.Time
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

func (r *PgxReservationRepository) Insert(ctx context.Context, sku uint64, count uint32, reservedUntil time.Time) (int64, error) {
	const query = `
INSERT INTO reservations (sku, count, reserved_until)
VALUES ($1, $2, $3)
RETURNING id`

	var id int64
	err := r.pool.QueryRow(ctx, query, int64(sku), int32(count), reservedUntil).Scan(&id)
	if err != nil {
		r.metrics.ReportRequest("InsertReservation", "error")
		return 0, fmt.Errorf("PgxReservationRepository.Insert: %w", err)
	}

	r.metrics.ReportRequest("InsertReservation", "success")
	return id, nil
}

func (r *PgxReservationRepository) GetByIds(ctx context.Context, ids []int64) ([]Reservation, error) {
	const query = `
SELECT id, sku, count, reserved_until
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
		if err = rows.Scan(&rv.Id, &sku, &count, &rv.ReservedUntil); err != nil {
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

func (r *PgxReservationRepository) GetExpired(ctx context.Context) ([]Reservation, error) {
	const query = `
SELECT id, sku, count, reserved_until
FROM reservations
WHERE reserved_until < NOW()`

	rows, err := r.pool.Query(ctx, query)
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
		if err = rows.Scan(&rv.Id, &sku, &count, &rv.ReservedUntil); err != nil {
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
