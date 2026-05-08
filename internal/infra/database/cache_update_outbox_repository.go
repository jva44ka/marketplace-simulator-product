package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

type CacheUpdateOutboxRepository struct {
	pool *pgxpool.Pool
}

func NewCacheUpdateOutboxRepository(pool *pgxpool.Pool) *CacheUpdateOutboxRepository {
	return &CacheUpdateOutboxRepository{pool: pool}
}

func (r *CacheUpdateOutboxRepository) GetPending(ctx context.Context, limit int) ([]models.CacheUpdateOutboxRecord, error) {
	const query = `
SELECT DISTINCT ON (sku)
    record_id, sku, created_at, retry_count, is_dead_letter,
    marked_as_dead_letter_at, dead_letter_reason
FROM outbox.cache_updates
WHERE is_dead_letter = FALSE
ORDER BY sku, created_at
LIMIT $1;`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("CacheUpdateOutboxRepository.GetPending: %w", err)
	}
	defer rows.Close()

	records := make([]models.CacheUpdateOutboxRecord, 0, limit)
	for rows.Next() {
		var rec models.CacheUpdateOutboxRecord
		if err = rows.Scan(
			&rec.RecordId, &rec.Sku, &rec.CreatedAt, &rec.RetryCount,
			&rec.IsDeadLetter, &rec.MarkedAsDeadLetterAt, &rec.DeadLetterReason,
		); err != nil {
			return nil, fmt.Errorf("CacheUpdateOutboxRepository.GetPending scan: %w", err)
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (r *CacheUpdateOutboxRepository) CountPending(ctx context.Context) (int64, error) {
	const query = `SELECT COUNT(*) FROM outbox.cache_updates WHERE is_dead_letter = FALSE;`
	var n int64
	if err := r.pool.QueryRow(ctx, query).Scan(&n); err != nil {
		return 0, fmt.Errorf("CacheUpdateOutboxRepository.CountPending: %w", err)
	}
	return n, nil
}

func (r *CacheUpdateOutboxRepository) CountDeadLetters(ctx context.Context) (int64, error) {
	const query = `SELECT COUNT(*) FROM outbox.cache_updates WHERE is_dead_letter = TRUE;`
	var n int64
	if err := r.pool.QueryRow(ctx, query).Scan(&n); err != nil {
		return 0, fmt.Errorf("CacheUpdateOutboxRepository.CountDeadLetters: %w", err)
	}
	return n, nil
}

func (r *CacheUpdateOutboxRepository) DeleteBatch(ctx context.Context, ids []uuid.UUID) error {
	const query = `DELETE FROM outbox.cache_updates WHERE record_id = ANY($1::uuid[]);`
	if _, err := r.pool.Exec(ctx, query, ids); err != nil {
		return fmt.Errorf("CacheUpdateOutboxRepository.DeleteBatch: %w", err)
	}
	return nil
}

func (r *CacheUpdateOutboxRepository) IncrementRetry(ctx context.Context, id uuid.UUID) error {
	const query = `UPDATE outbox.cache_updates SET retry_count = retry_count + 1 WHERE record_id = $1;`
	if _, err := r.pool.Exec(ctx, query, id); err != nil {
		return fmt.Errorf("CacheUpdateOutboxRepository.IncrementRetry: %w", err)
	}
	return nil
}

func (r *CacheUpdateOutboxRepository) MarkDeadLetter(ctx context.Context, id uuid.UUID, reason string) error {
	const query = `
UPDATE outbox.cache_updates
SET is_dead_letter = TRUE, marked_as_dead_letter_at = $2, dead_letter_reason = $3
WHERE record_id = $1;`
	if _, err := r.pool.Exec(ctx, query, id, time.Now(), reason); err != nil {
		return fmt.Errorf("CacheUpdateOutboxRepository.MarkDeadLetter: %w", err)
	}
	return nil
}

func (r *CacheUpdateOutboxRepository) WithTx(tx pgx.Tx) *CacheUpdateOutboxTxRepository {
	return &CacheUpdateOutboxTxRepository{tx: tx}
}

type CacheUpdateOutboxTxRepository struct {
	tx pgx.Tx
}

func (r *CacheUpdateOutboxTxRepository) Create(ctx context.Context, sku uint64) error {
	const query = `INSERT INTO outbox.cache_updates (sku) VALUES ($1);`
	if _, err := r.tx.Exec(ctx, query, sku); err != nil {
		return fmt.Errorf("CacheUpdateOutboxTxRepository.Create sku=%d: %w", sku, err)
	}
	return nil
}
