package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/kafka"
	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
	"github.com/jva44ka/ozon-simulator-go-products/internal/services"
	"github.com/jva44ka/ozon-simulator-go-products/internal/services/outbox"
)

// TODO: Вынести местную бизнес-логику в сервис, чтобы не было этой зависимости от репозитория
type ProductEventsOutboxReadRepository interface {
	GetPending(ctx context.Context, limit int) ([]models.ProductEventOutboxRecord, error)
	WithTx(tx pgx.Tx) services.ProductEventsOutboxWriteRepository
}

type ProductEventsOutboxWriteRepository interface {
	Create(ctx context.Context, record models.ProductEventOutboxRecordNew) error
	Delete(ctx context.Context, recordId uuid.UUID) error
	DeleteBatch(ctx context.Context, recordIds []uuid.UUID) error
	IncrementRetry(ctx context.Context, recordId uuid.UUID) error
	IncrementRetryBatch(ctx context.Context, recordIds []uuid.UUID) error
	MarkDeadLetter(ctx context.Context, recordId uuid.UUID, reason string) error
	MarkDeadLetterBatch(ctx context.Context, recordIds []uuid.UUID, reason string) error
}

type DBManager interface {
	ProductEventsOutboxRepo() ProductEventsOutboxReadRepository
	InTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type OutboxKafkaProducer interface {
	PublishProductChangedBatch(ctx context.Context, events []kafka.ProductChangedEvent) error
}

type OutboxPublisherJob struct {
	db            DBManager
	producer      OutboxKafkaProducer
	enabled       bool
	interval      time.Duration
	batchSize     int
	maxRetryCount int32
}

func NewOutboxPublisherJob(
	db DBManager,
	producer OutboxKafkaProducer,
	enabled bool,
	interval time.Duration,
	batchSize int,
	maxRetries int32,
) *OutboxPublisherJob {
	return &OutboxPublisherJob{
		db:            db,
		producer:      producer,
		enabled:       enabled,
		interval:      interval,
		batchSize:     batchSize,
		maxRetryCount: maxRetries,
	}
}

func (j *OutboxPublisherJob) Run(ctx context.Context) {
	if j.enabled {
		slog.InfoContext(ctx, "OutboxPublisherJob disabled, shutting down")
	}

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
	//TODO добавить метрики
	//TODO вынести логику обработки аутбокса отдельно от доменного процессинга
	//TODO сделать обработку паники
	outboxRecords, err := j.db.ProductEventsOutboxRepo().GetPending(ctx, j.batchSize)
	if err != nil {
		return fmt.Errorf("GetPending: %w", err)
	}

	if len(outboxRecords) == 0 {
		return nil
	}

	processBatchResult := j.processBatch(ctx, outboxRecords)

	outboxRecordsMap := make(map[uuid.UUID]models.ProductEventOutboxRecord)
	for _, outboxRecord := range outboxRecords {
		outboxRecordsMap[outboxRecord.RecordId] = outboxRecord
	}

	//TODO сделать батчевую обработку
	for failedRecordId, failedRecordReason := range processBatchResult.FailedRecordReasons {
		outboxRecord := outboxRecordsMap[failedRecordId]

		if outboxRecord.RetryCount+1 >= j.maxRetryCount {
			j.markAsDeadLetter(ctx, failedRecordId, failedRecordReason)
		} else {
			j.incrementRetry(ctx, failedRecordId)
		}
	}

	j.deleteRecords(ctx, processBatchResult.SuccessRecords)

	return nil
}

type FailedRecord struct {
	recordId uuid.UUID
	reason   string
}

type ProcessBatchResult struct {
	SuccessRecords      []uuid.UUID
	FailedRecordReasons map[uuid.UUID]string
}

// TODO: вынести метод с помощью композиции
func (j *OutboxPublisherJob) processBatch(ctx context.Context, records []models.ProductEventOutboxRecord) ProcessBatchResult {
	successRecords := make([]uuid.UUID, 0)
	failedRecordReasons := make(map[uuid.UUID]string)
	kafkaEvents := make([]kafka.ProductChangedEvent, 0, len(records))

	for _, outboxRecord := range records {
		//TODO: вынести маппинг
		var outboxRecordData outbox.ProductEventData
		if err := json.Unmarshal(outboxRecord.Data, &outboxRecordData); err != nil {
			failedRecordReasons[outboxRecord.RecordId] = err.Error()
			continue
		}

		kafkaEvents = append(kafkaEvents, kafka.ProductChangedEvent{
			RecordId: outboxRecord.RecordId,
			Old: kafka.ProductSnapshot{
				Sku:   outboxRecordData.Old.Sku,
				Name:  outboxRecordData.Old.Name,
				Price: outboxRecordData.Old.Price,
				Count: outboxRecordData.Old.Count,
			},
			New: kafka.ProductSnapshot{
				Sku:   outboxRecordData.New.Sku,
				Name:  outboxRecordData.New.Name,
				Price: outboxRecordData.New.Price,
				Count: outboxRecordData.New.Count,
			},
		})
	}

	if len(failedRecordReasons) == len(records) {
		return ProcessBatchResult{
			SuccessRecords:      successRecords,
			FailedRecordReasons: failedRecordReasons,
		}
	}

	if err := j.producer.PublishProductChangedBatch(ctx, kafkaEvents); err != nil {
		for _, kafkaEvent := range kafkaEvents {
			failedRecordReasons[kafkaEvent.RecordId] = err.Error()
		}
	} else {
		for _, kafkaEvent := range kafkaEvents {
			successRecords = append(successRecords, kafkaEvent.RecordId)
		}
	}

	return ProcessBatchResult{
		SuccessRecords:      successRecords,
		FailedRecordReasons: failedRecordReasons,
	}
}

func (j *OutboxPublisherJob) markAsDeadLetter(ctx context.Context, recordId uuid.UUID, reason string) {
	//TODO: чето сделать с этой ненужной вложенностью
	err := j.db.InTransaction(ctx, func(tx pgx.Tx) error {
		return j.db.ProductEventsOutboxRepo().WithTx(tx).MarkDeadLetter(
			ctx,
			recordId,
			reason)
	})
	if err != nil {
		slog.ErrorContext(ctx, "MarkDeadLetter failed with error", "err", err)
	}
}

func (j *OutboxPublisherJob) incrementRetry(ctx context.Context, recordId uuid.UUID) {
	//TODO: чето сделать с этой ненужной вложенностью
	err := j.db.InTransaction(ctx, func(tx pgx.Tx) error {
		return j.db.ProductEventsOutboxRepo().WithTx(tx).IncrementRetry(
			ctx,
			recordId)
	})
	if err != nil {
		slog.ErrorContext(ctx, "IncrementRetryBatch failed with error", "err", err)
	}
}

func (j *OutboxPublisherJob) deleteRecords(ctx context.Context, recordIds []uuid.UUID) {
	//TODO: чето сделать с этой ненужной вложенностью
	err := j.db.InTransaction(ctx, func(tx pgx.Tx) error {
		return j.db.ProductEventsOutboxRepo().WithTx(tx).DeleteBatch(
			ctx,
			recordIds)
	})
	if err != nil {
		slog.ErrorContext(ctx, "IncrementRetryBatch failed with error", "err", err)
	}
}
