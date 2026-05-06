package models

import (
	"time"

	"github.com/google/uuid"
)

// CacheUpdateOutboxRecord represents a pending Redis cache-refresh task.
// One record per modified SKU is written in the same DB transaction as
// the product mutation; a background job reads these records and updates Redis.
type CacheUpdateOutboxRecord struct {
	RecordId             uuid.UUID
	Sku                  uint64
	CreatedAt            time.Time
	RetryCount           int32
	IsDeadLetter         bool
	MarkedAsDeadLetterAt *time.Time
	DeadLetterReason     *string
}
