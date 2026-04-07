package models

import (
	"time"

	"github.com/google/uuid"
)

type ProductEventOutboxRecord struct {
	RecordId             uuid.UUID
	Key                  string
	Data                 []byte
	CreatedAt            time.Time
	RetryCount           int32
	IsDeadLetter         bool
	MarkedAsDeadLetterAt *time.Time
	DeadLetterReason     *string
}
