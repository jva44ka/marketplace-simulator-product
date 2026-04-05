package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
	"github.com/jva44ka/ozon-simulator-go-products/internal/services"
)

type ProductEventsOutboxPgxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxPgxRepository(pool *pgxpool.Pool) *ProductEventsOutboxPgxRepository {
	return &ProductEventsOutboxPgxRepository{pool: pool}
}

type OutboxPgxTxRepository struct {
	tx pgx.Tx
}

func (r *ProductEventsOutboxPgxRepository) WithTx(tx pgx.Tx) services.ProductEventsOutboxWriteRepository {
	return &OutboxPgxTxRepository{tx: tx}
}

func (r *OutboxPgxTxRepository) Create(ctx context.Context, record models.ProductEventOutboxRecordNew) error {
	const query = `
INSERT INTO outbox.product_events (key, data)
VALUES ($1, $2);`

	if _, err := r.tx.Exec(ctx, query, record.Key, record.Data); err != nil {
		return fmt.Errorf("OutboxRepository.Create: %w", err)
	}
	return nil
}

func (r *ProductEventsOutboxPgxRepository) GetPending(ctx context.Context, limit int) ([]models.ProductEventOutboxRecord, error) {
	const query = `
SELECT DISTINCT ON (key)
    record_id, key, data, created_at, retry_count, is_dead_letter, marked_as_dead_letter_at, dead_letter_reason
FROM outbox.product_events
WHERE is_dead_letter = FALSE
ORDER BY key, created_at
LIMIT $1;`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("OutboxRepository.GetPending: %w", err)
	}
	defer rows.Close()

	records := make([]models.ProductEventOutboxRecord, 0, limit)
	for rows.Next() {
		var e models.ProductEventOutboxRecord
		if err = rows.Scan(
			&e.RecordId, &e.Key, &e.Data, &e.CreatedAt,
			&e.RetryCount, &e.IsDeadLetter, &e.MarkedAsDeadLetterAt, &e.DeadLetterReason,
		); err != nil {
			return nil, fmt.Errorf("OutboxRepository.GetPending: scan: %w", err)
		}
		records = append(records, e)
	}
	return records, rows.Err()
}

func (r *OutboxPgxTxRepository) Delete(ctx context.Context, recordId string) error {
	const query = `
DELETE
FROM outbox.product_events
WHERE record_id = $1;`

	if _, err := r.tx.Exec(ctx, query, recordId); err != nil {
		return fmt.Errorf("OutboxRepository.Delete: %w", err)
	}
	return nil
}

func (r *OutboxPgxTxRepository) DeleteBatch(ctx context.Context, recordIds []string) error {
	const query = `DELETE FROM outbox.product_events WHERE record_id = ANY($1::uuid[]);`

	if _, err := r.tx.Exec(ctx, query, recordIds); err != nil {
		return fmt.Errorf("OutboxRepository.DeleteBatch: %w", err)
	}
	return nil
}

func (r *OutboxPgxTxRepository) IncrementRetry(ctx context.Context, recordId string) error {
	const query = `UPDATE outbox.product_events SET retry_count = retry_count + 1 WHERE record_id = $1;`

	if _, err := r.tx.Exec(ctx, query, recordId); err != nil {
		return fmt.Errorf("OutboxRepository.IncrementRetry: %w", err)
	}
	return nil
}

func (r *OutboxPgxTxRepository) IncrementRetryBatch(ctx context.Context, recordIds []string) error {
	const query = `UPDATE outbox.product_events SET retry_count = retry_count + 1 WHERE record_id = ANY($1::uuid[]);`

	if _, err := r.tx.Exec(ctx, query, recordIds); err != nil {
		return fmt.Errorf("OutboxRepository.IncrementRetryBatch: %w", err)
	}
	return nil
}

func (r *OutboxPgxTxRepository) MarkDeadLetter(ctx context.Context, recordId string, reason string) error {
	const query = `
UPDATE outbox.product_events
SET is_dead_letter = TRUE,
    marked_as_dead_letter_at = $2,
    dead_letter_reason = $3
WHERE record_id = $1;`

	if _, err := r.tx.Exec(ctx, query, recordId, time.Now(), reason); err != nil {
		return fmt.Errorf("OutboxRepository.MarkDeadLetter: %w", err)
	}
	return nil
}

func (r *OutboxPgxTxRepository) MarkDeadLetterBatch(ctx context.Context, recordIds []string, reason string) error {
	const query = `
UPDATE outbox.product_events
SET is_dead_letter = TRUE,
    marked_as_dead_letter_at = $2,
    dead_letter_reason = $3
WHERE record_id = ANY($1::uuid[]);`

	if _, err := r.tx.Exec(ctx, query, recordIds, time.Now(), reason); err != nil {
		return fmt.Errorf("OutboxRepository.MarkDeadLetterBatch: %w", err)
	}
	return nil
}
