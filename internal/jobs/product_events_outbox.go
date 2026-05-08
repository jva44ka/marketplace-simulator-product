package jobs

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	kafkaContracts "github.com/jva44ka/marketplace-simulator-product/api_internal/kafka"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/config"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

type OutboxKafkaProducer interface {
	PublishProductChangedBatch(ctx context.Context, events []kafkaContracts.ProductEventMessage) error
}

type OutboxJobMetrics interface {
	ReportProcessed(status string, count int)
	ReportTickDuration(d time.Duration)
	ReportKafkaPublishDuration(d time.Duration)
	ReportRecordAge(age time.Duration)
}

type ProductEventsOutboxRepo interface {
	GetPending(ctx context.Context, limit int) ([]models.ProductEventOutboxRecord, error)
	GetCount(ctx context.Context, isDeadLetter bool) (int64, error)
	DeleteBatch(ctx context.Context, recordIds []uuid.UUID) error
	IncrementRetry(ctx context.Context, recordId uuid.UUID) error
	MarkDeadLetter(ctx context.Context, recordId uuid.UUID, reason string) error
}

type ProductEventsOutboxJob struct {
	outboxRepo ProductEventsOutboxRepo
	producer   OutboxKafkaProducer
	metrics    OutboxJobMetrics
	cfgStore   *config.ConfigStore
}

func NewProductEventsOutboxJob(
	outboxRepo ProductEventsOutboxRepo,
	producer OutboxKafkaProducer,
	metrics OutboxJobMetrics,
	cfgStore *config.ConfigStore,
) *ProductEventsOutboxJob {
	return &ProductEventsOutboxJob{
		outboxRepo: outboxRepo,
		producer:   producer,
		metrics:    metrics,
		cfgStore:   cfgStore,
	}
}

func (j *ProductEventsOutboxJob) Run(ctx context.Context) {
	lastProcessed := 0

	for {
		cfg := j.cfgStore.Load().Jobs.ProductEventsOutbox

		//todo рефакторинг, чтобы не парсить на каждую итерацию
		idleInterval, err := time.ParseDuration(cfg.IdleInterval)
		if err != nil {
			idleInterval = 100 * time.Millisecond
			slog.Warn("ProductEventsOutboxJob: invalid idle-interval, using 100ms", "err", err)
		}

		activeInterval, err := time.ParseDuration(cfg.ActiveInterval)
		if err != nil {
			activeInterval = 0
			slog.Warn("ProductEventsOutboxJob: invalid active-interval, using 0", "err", err)
		}

		interval := idleInterval
		if lastProcessed > 0 {
			interval = activeInterval
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		if !cfg.Enabled {
			lastProcessed = 0
			continue
		}

		lastProcessed = j.tick(ctx, cfg.BatchSize, int32(cfg.MaxRetries))
	}
}

func (j *ProductEventsOutboxJob) tick(ctx context.Context, batchSize int, maxRetryCount int32) int {
	tickStart := time.Now()
	defer func() {
		j.metrics.ReportTickDuration(time.Since(tickStart))
	}()

	outboxRecords, err := j.outboxRepo.GetPending(ctx, batchSize)
	if err != nil {
		slog.ErrorContext(ctx, "ProductEventsOutboxJob: GetPending failed", "err", err)
		return 0
	}

	if len(outboxRecords) == 0 {
		return 0
	}

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

		if outboxRecord.RetryCount+1 >= maxRetryCount {
			err = j.outboxRepo.MarkDeadLetter(ctx, failedRecordId, failedRecordReason)
			if err != nil {
				slog.ErrorContext(ctx, "MarkDeadLetter failed with error", "err", err)
			}
			deadLetterCount++
		} else {
			err = j.outboxRepo.IncrementRetry(ctx, failedRecordId)
			if err != nil {
				slog.ErrorContext(ctx, "IncrementRetry failed with error", "err", err)
			}
			failedCount++
		}
	}

	err = j.outboxRepo.DeleteBatch(ctx, processBatchResult.SuccessRecords)
	if err != nil {
		slog.ErrorContext(ctx, "DeleteBatch failed with error", "err", err)
	}

	j.metrics.ReportProcessed("success", len(processBatchResult.SuccessRecords))
	j.metrics.ReportProcessed("failed", failedCount)
	j.metrics.ReportProcessed("dead_letter", deadLetterCount)

	return len(outboxRecords)
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
