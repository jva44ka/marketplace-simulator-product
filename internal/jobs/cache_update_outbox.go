package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/config"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/jva44ka/marketplace-simulator-product/internal/services"
)

type CacheUpdateDBManager interface {
	CacheUpdateOutboxRepo() services.CacheUpdateOutboxRepository
}

type CacheUpdateProductRepo interface {
	GetBySku(ctx context.Context, sku uint64, txId *uint32) (*models.Product, error)
}

type CacheWriter interface {
	Set(ctx context.Context, product *models.Product)
}

type CacheUpdateOutboxJobMetrics interface {
	ReportProcessed(status string, count int)
	ReportTickDuration(d time.Duration)
	ReportRecordAge(age time.Duration)
}

type CacheUpdateOutboxJob struct {
	db          CacheUpdateDBManager
	productRepo CacheUpdateProductRepo
	cache       CacheWriter // nil when cache is disabled
	metrics     CacheUpdateOutboxJobMetrics
	cfgStore    *config.ConfigStore
}

func NewCacheUpdateOutboxJob(
	db CacheUpdateDBManager,
	productRepo CacheUpdateProductRepo,
	cache CacheWriter,
	metrics CacheUpdateOutboxJobMetrics,
	cfgStore *config.ConfigStore,
) *CacheUpdateOutboxJob {
	return &CacheUpdateOutboxJob{
		db:          db,
		productRepo: productRepo,
		cache:       cache,
		metrics:     metrics,
		cfgStore:    cfgStore,
	}
}

func (j *CacheUpdateOutboxJob) Run(ctx context.Context) {
	lastProcessed := 0

	for {
		cfg := j.cfgStore.Load().Jobs.CacheUpdateOutbox

		idleInterval, err := time.ParseDuration(cfg.IdleInterval)
		if err != nil {
			idleInterval = 100 * time.Millisecond
			slog.Warn("CacheUpdateOutboxJob: invalid idle-interval, using 100ms", "err", err)
		}

		activeInterval, err := time.ParseDuration(cfg.ActiveInterval)
		if err != nil {
			activeInterval = 0
			slog.Warn("CacheUpdateOutboxJob: invalid active-interval, using 0", "err", err)
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

func (j *CacheUpdateOutboxJob) tick(ctx context.Context, batchSize int, maxRetries int32) int {
	if j.cache != nil {
		slog.ErrorContext(ctx, "CacheUpdateOutboxJob: cache is null. Redis is required for CacheUpdateOutboxJob")
		return 0
	}

	tickStart := time.Now()
	defer func() {
		j.metrics.ReportTickDuration(time.Since(tickStart))
	}()

	records, err := j.db.CacheUpdateOutboxRepo().GetPending(ctx, batchSize)
	if err != nil {
		slog.ErrorContext(ctx, "CacheUpdateOutboxJob: GetPending failed", "err", err)
		return 0
	}
	if len(records) == 0 {
		return 0
	}

	for _, rec := range records {
		j.metrics.ReportRecordAge(time.Since(rec.CreatedAt))
	}

	result := j.processBatch(ctx, records)

	recordsByID := make(map[uuid.UUID]models.CacheUpdateOutboxRecord, len(records))
	for _, r := range records {
		recordsByID[r.RecordId] = r
	}

	deadLetterCount := 0
	failedCount := 0

	for failedID, reason := range result.FailedRecordReasons {
		rec := recordsByID[failedID]
		if rec.RetryCount+1 >= maxRetries {
			if err = j.db.CacheUpdateOutboxRepo().MarkDeadLetter(ctx, failedID, reason); err != nil {
				slog.ErrorContext(ctx, "CacheUpdateOutboxJob: MarkDeadLetter failed", "err", err)
			}
			deadLetterCount++
		} else {
			if err = j.db.CacheUpdateOutboxRepo().IncrementRetry(ctx, failedID); err != nil {
				slog.ErrorContext(ctx, "CacheUpdateOutboxJob: IncrementRetry failed", "err", err)
			}
			failedCount++
		}
	}

	if err = j.db.CacheUpdateOutboxRepo().DeleteBatch(ctx, result.SuccessRecords); err != nil {
		slog.ErrorContext(ctx, "CacheUpdateOutboxJob: DeleteBatch failed", "err", err)
	}

	j.metrics.ReportProcessed("success", len(result.SuccessRecords))
	j.metrics.ReportProcessed("failed", failedCount)
	j.metrics.ReportProcessed("dead_letter", deadLetterCount)

	return len(records)
}

func (j *CacheUpdateOutboxJob) processBatch(ctx context.Context, records []models.CacheUpdateOutboxRecord) ProcessBatchResult {
	successRecords := make([]uuid.UUID, 0, len(records))
	failedRecordReasons := make(map[uuid.UUID]string)

	for _, rec := range records {
		product, err := j.productRepo.GetBySku(ctx, rec.Sku, nil)
		if err != nil {
			slog.ErrorContext(ctx, "CacheUpdateOutboxJob: GetBySku failed",
				"sku", rec.Sku, "record_id", rec.RecordId, "err", err)
			failedRecordReasons[rec.RecordId] = err.Error()
			continue
		}

		j.cache.Set(ctx, product)
		successRecords = append(successRecords, rec.RecordId)
	}

	return ProcessBatchResult{
		SuccessRecords:      successRecords,
		FailedRecordReasons: failedRecordReasons,
	}
}
