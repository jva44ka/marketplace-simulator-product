package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	kafkaContracts "github.com/jva44ka/ozon-simulator-go-products/api_internal/kafka"
	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
	"github.com/jva44ka/ozon-simulator-go-products/internal/services"
)

type DBManager interface {
	ProductEventsOutboxRepo() services.ProductEventsOutboxRepository
	InTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type OutboxKafkaProducer interface {
	PublishProductChangedBatch(ctx context.Context, events []kafkaContracts.ProductEventMessage) error
}

type OutboxJobMetrics interface {
	ReportProcessed(status string, count int)
	ReportTickDuration(d time.Duration)
	ReportKafkaPublishDuration(d time.Duration)
	ReportRecordAge(age time.Duration)
}

type ProductEventsOutboxJob struct {
	db            DBManager
	producer      OutboxKafkaProducer
	metrics       OutboxJobMetrics
	enabled       bool
	interval      time.Duration
	batchSize     int
	maxRetryCount int32
}

func NewProductEventsOutboxJob(
	db DBManager,
	producer OutboxKafkaProducer,
	metrics OutboxJobMetrics,
	enabled bool,
	interval time.Duration,
	batchSize int,
	maxRetries int32,
) *ProductEventsOutboxJob {
	return &ProductEventsOutboxJob{
		db:            db,
		producer:      producer,
		metrics:       metrics,
		enabled:       enabled,
		interval:      interval,
		batchSize:     batchSize,
		maxRetryCount: maxRetries,
	}
}

func (j *ProductEventsOutboxJob) Run(ctx context.Context) {
	if !j.enabled {
		slog.InfoContext(ctx, "ProductEventsOutboxJob disabled, shutting down")
		return
	}

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := j.tick(ctx); err != nil {
				slog.ErrorContext(ctx, "product events outbox job failed", "err", err)
			}
		}
	}
}

func (j *ProductEventsOutboxJob) tick(ctx context.Context) error {
	tickStart := time.Now()
	defer func() {
		j.metrics.ReportTickDuration(time.Since(tickStart))
	}()

	outboxRecords, err := j.db.ProductEventsOutboxRepo().GetPending(ctx, j.batchSize)
	if err != nil {
		return fmt.Errorf("GetPending: %w", err)
	}

	if len(outboxRecords) == 0 {
		return nil
	}

	// Замеряем возраст записей
	for _, record := range outboxRecords {
		j.metrics.ReportRecordAge(time.Since(record.CreatedAt))
	}

	processBatchResult := j.processBatch(ctx, outboxRecords)

	outboxRecordsMap := make(map[uuid.UUID]models.ProductEventOutboxRecord)
	for _, outboxRecord := range outboxRecords {
		outboxRecordsMap[outboxRecord.RecordId] = outboxRecord
	}

	deadLetterCount := 0
	failedCount := 0

	//TODO сделать батчевую обработку
	for failedRecordId, failedRecordReason := range processBatchResult.FailedRecordReasons {
		outboxRecord := outboxRecordsMap[failedRecordId]

		if outboxRecord.RetryCount+1 >= j.maxRetryCount {
			err = j.db.ProductEventsOutboxRepo().MarkDeadLetter(
				ctx,
				failedRecordId,
				failedRecordReason)
			if err != nil {
				slog.ErrorContext(ctx, "MarkDeadLetter failed with error", "err", err)
			}
			deadLetterCount++
		} else {
			err = j.db.ProductEventsOutboxRepo().IncrementRetry(
				ctx,
				failedRecordId)
			if err != nil {
				slog.ErrorContext(ctx, "IncrementRetry failed with error", "err", err)
			}
			failedCount++
		}
	}

	err = j.db.ProductEventsOutboxRepo().DeleteBatch(
		ctx,
		processBatchResult.SuccessRecords)
	if err != nil {
		slog.ErrorContext(ctx, "DeleteBatch failed with error", "err", err)
	}

	// Репортим метрики по результатам
	if len(processBatchResult.SuccessRecords) > 0 {
		j.metrics.ReportProcessed("success", len(processBatchResult.SuccessRecords))
	}
	if failedCount > 0 {
		j.metrics.ReportProcessed("failed", failedCount)
	}
	if deadLetterCount > 0 {
		j.metrics.ReportProcessed("dead_letter", deadLetterCount)
	}

	return nil
}

type ProcessBatchResult struct {
	SuccessRecords      []uuid.UUID
	FailedRecordReasons map[uuid.UUID]string
}

// TODO: вынести метод с помощью композиции
func (j *ProductEventsOutboxJob) processBatch(ctx context.Context, records []models.ProductEventOutboxRecord) (result ProcessBatchResult) {
	successRecords := make([]uuid.UUID, 0)
	failedRecordReasons := make(map[uuid.UUID]string)

	kafkaEvents := make([]kafkaContracts.ProductEventMessage, 0, len(records))

	for _, outboxRecord := range records {
		//TODO: вынести маппинг
		var outboxRecordData kafkaContracts.ProductEventData
		if err := json.Unmarshal(outboxRecord.Data, &outboxRecordData); err != nil {
			failedRecordReasons[outboxRecord.RecordId] = err.Error()
			continue
		}

		kafkaEvents = append(kafkaEvents, kafkaContracts.ProductEventMessage{
			Key: outboxRecordData.New.Sku,
			Body: kafkaContracts.ProductEventBody{
				RecordId: outboxRecord.RecordId,
				Data:     outboxRecordData,
			},
			Headers: outboxRecord.Headers,
		})
	}

	if len(failedRecordReasons) == len(records) {
		return ProcessBatchResult{
			SuccessRecords:      successRecords,
			FailedRecordReasons: failedRecordReasons,
		}
	}

	publishStart := time.Now()
	err := j.producer.PublishProductChangedBatch(ctx, kafkaEvents)
	j.metrics.ReportKafkaPublishDuration(time.Since(publishStart))

	if err != nil {
		for _, kafkaEvent := range kafkaEvents {
			failedRecordReasons[kafkaEvent.Body.RecordId] = err.Error()
		}
	} else {
		for _, kafkaEvent := range kafkaEvents {
			successRecords = append(successRecords, kafkaEvent.Body.RecordId)
		}
	}

	return ProcessBatchResult{
		SuccessRecords:      successRecords,
		FailedRecordReasons: failedRecordReasons,
	}
}
