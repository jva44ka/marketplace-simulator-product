package models

import "time"

type ProductEventOutboxRecord struct {
	RecordId             string
	Key                  string
	Data                 []byte
	CreatedAt            time.Time
	RetryCount           int32
	IsDeadLetter         bool
	MarkedAsDeadLetterAt *time.Time
	DeadLetterReason     *string
}
