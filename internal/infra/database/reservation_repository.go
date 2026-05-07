package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/jva44ka/marketplace-simulator-product/internal/services"
)

type ReservationRepositoryMetrics interface {
	ReportRequest(method, status string, duration time.Duration)
}

type ReservationPgxRepository struct {
	pool    *pgxpool.Pool
	metrics ReservationRepositoryMetrics
}

func NewReservationPgxRepository(pool *pgxpool.Pool, metrics ReservationRepositoryMetrics) *ReservationPgxRepository {
	return &ReservationPgxRepository{pool: pool, metrics: metrics}
}

func (r *ReservationPgxRepository) GetByIds(ctx context.Context, ids []int64) ([]models.Reservation, error) {
	const query = `
SELECT id, sku, count, created_at
FROM reservations
WHERE id = ANY($1)`

	start := time.Now()
	rows, err := r.pool.Query(ctx, query, ids)
	if err != nil {
		r.metrics.ReportRequest("GetReservationsByIds", "error", time.Since(start))
		return nil, fmt.Errorf("ReservationPgxRepository.GetByIds: %w", err)
	}
	defer rows.Close()

	var result []models.Reservation
	for rows.Next() {
		var reservation models.Reservation
		var sku int64
		var count int32
		if err = rows.Scan(&reservation.Id, &sku, &count, &reservation.CreatedAt); err != nil {
			r.metrics.ReportRequest("GetReservationsByIds", "error", time.Since(start))
			return nil, fmt.Errorf("ReservationPgxRepository.GetByIds: %w", err)
		}
		reservation.Sku = uint64(sku)
		reservation.Count = uint32(count)
		result = append(result, reservation)
	}

	r.metrics.ReportRequest("GetReservationsByIds", "success", time.Since(start))
	return result, nil
}

func (r *ReservationPgxRepository) GetExpired(ctx context.Context, cutoff time.Time) ([]models.Reservation, error) {
	const query = `
SELECT id, sku, count, created_at
FROM reservations
WHERE created_at < $1`

	start := time.Now()
	rows, err := r.pool.Query(ctx, query, cutoff)
	if err != nil {
		r.metrics.ReportRequest("GetExpiredReservations", "error", time.Since(start))
		return nil, fmt.Errorf("ReservationPgxRepository.GetExpired: %w", err)
	}
	defer rows.Close()

	var result []models.Reservation
	for rows.Next() {
		var rv models.Reservation
		var sku int64
		var count int32
		if err = rows.Scan(&rv.Id, &sku, &count, &rv.CreatedAt); err != nil {
			r.metrics.ReportRequest("GetExpiredReservations", "error", time.Since(start))
			return nil, fmt.Errorf("ReservationPgxRepository.GetExpired: %w", err)
		}
		rv.Sku = uint64(sku)
		rv.Count = uint32(count)
		result = append(result, rv)
	}

	r.metrics.ReportRequest("GetExpiredReservations", "success", time.Since(start))
	return result, nil
}

func (r *ReservationPgxRepository) WithTx(tx pgx.Tx) services.ReservationTxRepository {
	return &ReservationPgxTxRepository{tx: tx, metrics: r.metrics}
}

type ReservationPgxTxRepository struct {
	tx      pgx.Tx
	metrics ReservationRepositoryMetrics
}

func (r *ReservationPgxTxRepository) Insert(ctx context.Context, sku uint64, count uint32) (models.Reservation, error) {
	const query = `
INSERT INTO reservations (sku, count)
VALUES ($1, $2)
RETURNING id, sku, count, created_at`

	start := time.Now()
	var reservation models.Reservation
	var skuInt int64
	var countInt int32
	err := r.tx.QueryRow(ctx, query, int64(sku), int32(count)).Scan(
		&reservation.Id, &skuInt, &countInt, &reservation.CreatedAt,
	)
	if err != nil {
		r.metrics.ReportRequest("InsertReservation", "error", time.Since(start))
		return models.Reservation{}, fmt.Errorf("ReservationPgxTxRepository.Insert: %w", err)
	}
	reservation.Sku = uint64(skuInt)
	reservation.Count = uint32(countInt)

	r.metrics.ReportRequest("InsertReservation", "success", time.Since(start))
	return reservation, nil
}

func (r *ReservationPgxTxRepository) DeleteByIds(ctx context.Context, ids []int64) error {
	const query = `DELETE FROM reservations WHERE id = ANY($1)`

	start := time.Now()
	_, err := r.tx.Exec(ctx, query, ids)
	if err != nil {
		r.metrics.ReportRequest("DeleteReservationsByIds", "error", time.Since(start))
		return fmt.Errorf("ReservationPgxTxRepository.DeleteByIds: %w", err)
	}

	r.metrics.ReportRequest("DeleteReservationsByIds", "success", time.Since(start))
	return nil
}
