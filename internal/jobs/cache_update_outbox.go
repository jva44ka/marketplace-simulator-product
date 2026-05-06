package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/cache"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/config"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/jva44ka/marketplace-simulator-product/internal/services"
)

// CacheProductRepository is the minimal read interface needed by the job.
type CacheProductRepository interface {
	GetBySku(ctx context.Context, sku uint64) (*models.Product, error)
}

// CacheUpdateOutboxJob polls outbox.cache_updates and refreshes Redis
// for each modified product. Uses the same idle/active interval pattern
// as ProductEventsOutboxJob.
type CacheUpdateOutboxJob struct {
	outboxRepo  services.CacheUpdateOutboxRepository
	productRepo CacheProductRepository
	cache       *cache.ProductCache
	cfgStore    *config.ConfigStore
}

func NewCacheUpdateOutboxJob(
	outboxRepo services.CacheUpdateOutboxRepository,
	productRepo CacheProductRepository,
	cache *cache.ProductCache,
	cfgStore *config.ConfigStore,
) *CacheUpdateOutboxJob {
	return &CacheUpdateOutboxJob{
		outboxRepo:  outboxRepo,
		productRepo: productRepo,
		cache:       cache,
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
	records, err := j.outboxRepo.GetPending(ctx, batchSize)
	if err != nil {
		slog.ErrorContext(ctx, "CacheUpdateOutboxJob: GetPending failed", "err", err)
		return 0
	}
	if len(records) == 0 {
		return 0
	}

	successIDs := make([]uuid.UUID, 0, len(records))
	recordsByID := make(map[uuid.UUID]models.CacheUpdateOutboxRecord, len(records))
	for _, r := range records {
		recordsByID[r.RecordId] = r
	}

	failedReasons := make(map[uuid.UUID]string)

	for _, rec := range records {
		product, err := j.productRepo.GetBySku(ctx, rec.Sku)
		if err != nil {
			slog.ErrorContext(ctx, "CacheUpdateOutboxJob: GetBySku failed",
				"sku", rec.Sku, "record_id", rec.RecordId, "err", err)
			failedReasons[rec.RecordId] = err.Error()
			continue
		}

		if j.cache == nil {
			// Cache disabled — just delete the outbox record so it doesn't pile up.
			successIDs = append(successIDs, rec.RecordId)
			continue
		}

		j.cache.Set(ctx, &cache.CachedProduct{
			Sku:           product.Sku,
			Name:          product.Name,
			Price:         product.Price,
			Count:         product.Count,
			ReservedCount: product.ReservedCount,
			TransactionId: product.TransactionId,
		})

		successIDs = append(successIDs, rec.RecordId)
	}

	// Handle failures: increment retry or mark dead letter.
	for id, reason := range failedReasons {
		rec := recordsByID[id]
		if rec.RetryCount+1 >= maxRetries {
			if err = j.outboxRepo.MarkDeadLetter(ctx, id, reason); err != nil {
				slog.ErrorContext(ctx, "CacheUpdateOutboxJob: MarkDeadLetter failed", "err", err)
			}
		} else {
			if err = j.outboxRepo.IncrementRetry(ctx, id); err != nil {
				slog.ErrorContext(ctx, "CacheUpdateOutboxJob: IncrementRetry failed", "err", err)
			}
		}
	}

	if len(successIDs) > 0 {
		if err = j.outboxRepo.DeleteBatch(ctx, successIDs); err != nil {
			slog.ErrorContext(ctx, "CacheUpdateOutboxJob: DeleteBatch failed", "err", err)
		}
	}

	slog.Info("CacheUpdateOutboxJob: tick done",
		"processed", len(records),
		"success", len(successIDs),
		"failed", len(failedReasons))

	return len(records)
}
