package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/jva44ka/marketplace-simulator-product/internal/services"
)

type ProductEventsOutboxRepository struct {
	pool *pgxpool.Pool
}

func NewProductEventsOutboxRepository(pool *pgxpool.Pool) *ProductEventsOutboxRepository {
	return &ProductEventsOutboxRepository{pool: pool}
}

func (r *ProductEventsOutboxRepository) GetPending(ctx context.Context, limit int) ([]models.ProductEventOutboxRecord, error) {
	const query = `
SELECT DISTINCT ON (key)
    record_id, key, data, headers, created_at, retry_count, is_dead_letter, marked_as_dead_letter_at, dead_letter_reason
FROM outbox.product_events
WHERE is_dead_letter = FALSE
ORDER BY key, created_at
LIMIT $1;`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("ProductEventsOutboxRepository.GetPending: %w", err)
	}
	defer rows.Close()

	records := make([]models.ProductEventOutboxRecord, 0, limit)
	for rows.Next() {
		var e models.ProductEventOutboxRecord
		if err = rows.Scan(
			&e.RecordId, &e.Key, &e.Data, &e.Headers, &e.CreatedAt,
			&e.RetryCount, &e.IsDeadLetter, &e.MarkedAsDeadLetterAt, &e.DeadLetterReason,
		); err != nil {
			return nil, fmt.Errorf("ProductEventsOutboxRepository.GetPending scan: %w", err)
		}
		records = append(records, e)
	}
	return records, rows.Err()
}

func (r *ProductEventsOutboxRepository) GetCount(ctx context.Context, isDeadLetter bool) (int64, error) {
	const query = `
SELECT COUNT(*)
FROM outbox.product_events
WHERE is_dead_letter = $1;`

	var count int64
	if err := r.pool.QueryRow(ctx, query, isDeadLetter).Scan(&count); err != nil {
		return 0, fmt.Errorf("ProductEventsOutboxRepository.GetCount: %w", err)
	}
	return count, nil
}

func (r *ProductEventsOutboxRepository) DeleteBatch(ctx context.Context, recordIds []uuid.UUID) error {
	const query = `DELETE FROM outbox.product_events WHERE record_id = ANY($1::uuid[]);`

	if _, err := r.pool.Exec(ctx, query, recordIds); err != nil {
		return fmt.Errorf("ProductEventsOutboxRepository.DeleteBatch: %w", err)
	}
	return nil
}

func (r *ProductEventsOutboxRepository) IncrementRetry(ctx context.Context, recordId uuid.UUID) error {
	const query = `UPDATE outbox.product_events SET retry_count = retry_count + 1 WHERE record_id = $1;`

	if _, err := r.pool.Exec(ctx, query, recordId); err != nil {
		return fmt.Errorf("ProductEventsOutboxRepository.IncrementRetry: %w", err)
	}
	return nil
}

func (r *ProductEventsOutboxRepository) MarkDeadLetter(ctx context.Context, recordId uuid.UUID, reason string) error {
	const query = `
UPDATE outbox.product_events
SET is_dead_letter = TRUE,
    marked_as_dead_letter_at = $2,
    dead_letter_reason = $3
WHERE record_id = $1;`

	if _, err := r.pool.Exec(ctx, query, recordId, time.Now(), reason); err != nil {
		return fmt.Errorf("ProductEventsOutboxRepository.MarkDeadLetter: %w", err)
	}
	return nil
}

func (r *ProductEventsOutboxRepository) WithTx(tx pgx.Tx) services.ProductEventsOutboxTxRepository {
	return &ProductEventsOutboxTxRepository{tx: tx}
}

type ProductEventsOutboxTxRepository struct {
	tx pgx.Tx
}

func (r *ProductEventsOutboxTxRepository) Create(ctx context.Context, record models.ProductEventOutboxRecordNew) error {
	const query = `
INSERT INTO outbox.product_events (key, data, headers)
VALUES ($1, $2, $3);`

	if _, err := r.tx.Exec(ctx, query, record.Key, record.Data, record.Headers); err != nil {
		return fmt.Errorf("ProductEventsOutboxTxRepository.Create: %w", err)
	}
	return nil
}
