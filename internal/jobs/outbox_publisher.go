package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/kafka"
	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
)

type OutboxRepository interface {
	GetPending(ctx context.Context, limit int) ([]models.ProductEventOutboxRecord, error)
	Delete(ctx context.Context, recordId string) error
	DeleteBatch(ctx context.Context, recordIds []string) error
	IncrementRetry(ctx context.Context, recordId string) error
	IncrementRetryBatch(ctx context.Context, recordIds []string) error
	MarkDeadLetter(ctx context.Context, recordId string, reason string) error
	MarkDeadLetterBatch(ctx context.Context, recordIds []string, reason string) error
}

type OutboxKafkaProducer interface {
	PublishProductChangedBatch(ctx context.Context, events []kafka.ProductChangedEvent) error
}

type OutboxPublisherJob struct {
	repo          OutboxRepository
	producer      OutboxKafkaProducer
	interval      time.Duration
	batchSize     int
	maxRetryCount int32
}

func NewOutboxPublisherJob(
	repo OutboxRepository,
	producer OutboxKafkaProducer,
	interval time.Duration,
	batchSize int,
	maxRetries int32,
) *OutboxPublisherJob {
	return &OutboxPublisherJob{
		repo:          repo,
		producer:      producer,
		interval:      interval,
		batchSize:     batchSize,
		maxRetryCount: maxRetries,
	}
}

func (j *OutboxPublisherJob) Run(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := j.tick(ctx); err != nil {
				slog.ErrorContext(ctx, "outbox publisher job failed", "err", err)
			}
		}
	}
}

func (j *OutboxPublisherJob) tick(ctx context.Context) error {
	outboxRecords, err := j.repo.GetPending(ctx, j.batchSize)
	if err != nil {
		return fmt.Errorf("GetPending: %w", err)
	}
	if len(outboxRecords) == 0 {
		return nil
	}

	var (
		validRecords  []models.ProductEventOutboxRecord
		validEvents   []kafka.ProductChangedEvent
		deadLetterIds []string
	)

	for _, r := range outboxRecords {
		var payload kafka.ProductChangedEvent
		if err := json.Unmarshal(r.Data, &payload); err != nil {
			slog.ErrorContext(ctx, "unmarshal failed, marking as dead letter",
				"record_id", r.RecordId, "err", err)
			deadLetterIds = append(deadLetterIds, r.RecordId)
			continue
		}
		validRecords = append(validRecords, r)
		validEvents = append(validEvents, payload)
	}

	if len(deadLetterIds) > 0 {
		if dlErr := j.repo.MarkDeadLetterBatch(ctx, deadLetterIds, "unmarshal failed"); dlErr != nil {
			slog.ErrorContext(ctx, "MarkDeadLetterBatch (unmarshal) failed", "err", dlErr)
		}
	}

	if len(validEvents) == 0 {
		return nil
	}

	if err := j.producer.PublishProductChangedBatch(ctx, validEvents); err != nil {
		slog.ErrorContext(ctx, "PublishProductChangedBatch failed", "err", err)

		var retryIds, maxReachedIds []string
		for _, r := range validRecords {
			if r.RetryCount+1 >= j.maxRetryCount {
				maxReachedIds = append(maxReachedIds, r.RecordId)
			} else {
				retryIds = append(retryIds, r.RecordId)
			}
		}

		if len(maxReachedIds) > 0 {
			reason := fmt.Sprintf("max retries reached: %s", err)
			if dlErr := j.repo.MarkDeadLetterBatch(ctx, maxReachedIds, reason); dlErr != nil {
				slog.ErrorContext(ctx, "MarkDeadLetterBatch (max retries) failed", "err", dlErr)
			}
		}
		if len(retryIds) > 0 {
			if retryErr := j.repo.IncrementRetryBatch(ctx, retryIds); retryErr != nil {
				slog.ErrorContext(ctx, "IncrementRetryBatch failed", "err", retryErr)
			}
		}

		return fmt.Errorf("PublishProductChangedBatch: %w", err)
	}

	successIds := make([]string, 0, len(validRecords))
	for _, r := range validRecords {
		successIds = append(successIds, r.RecordId)
	}
	return j.repo.DeleteBatch(ctx, successIds)
}
