package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jva44ka/marketplace-simulator-product/internal/infra/config"
)

type MetricCollectorRepository interface {
	GetCount(ctx context.Context, isDeadLetter bool) (int64, error)
}

type CacheUpdateMetricCollectorRepository interface {
	CountPending(ctx context.Context) (int64, error)
	CountDeadLetters(ctx context.Context) (int64, error)
}

type MetricCollectorMetrics interface {
	SetPending(outboxName string, count int64)
	SetDeadLetter(outboxName string, count int64)
	SetAcquiredConns(n int32)
	SetIdleConns(n int32)
	SetTotalConns(n int32)
	SetMaxConns(n int32)
	SetAvgAcquireDuration(d time.Duration)
}

type MetricCollectorJob struct {
	productEventsRepo   MetricCollectorRepository
	cacheUpdateRepo     CacheUpdateMetricCollectorRepository
	pool                *pgxpool.Pool
	metrics             MetricCollectorMetrics
	cfgStore            *config.ConfigStore
	prevAcquireCount    int64
	prevAcquireDuration time.Duration
}

func NewMetricCollectorJob(
	productEventsRepo MetricCollectorRepository,
	cacheUpdateRepo CacheUpdateMetricCollectorRepository,
	pool *pgxpool.Pool,
	metrics MetricCollectorMetrics,
	cfgStore *config.ConfigStore,
) *MetricCollectorJob {
	return &MetricCollectorJob{
		productEventsRepo: productEventsRepo,
		cacheUpdateRepo:   cacheUpdateRepo,
		pool:              pool,
		metrics:           metrics,
		cfgStore:          cfgStore,
	}
}

func (j *MetricCollectorJob) Run(ctx context.Context) {
	for {
		cfg := j.cfgStore.Load().Jobs.ProductEventsOutboxMonitor

		//todo рефакторинг, чтобы не парсить на каждую итерацию
		interval, err := time.ParseDuration(cfg.JobInterval)
		if err != nil {
			interval = 10 * time.Second
			slog.Warn("MetricCollectorJob: invalid job-interval, using 10s", "err", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		if !cfg.Enabled {
			continue
		}

		j.tick(ctx)
	}
}

func (j *MetricCollectorJob) tick(ctx context.Context) {
	// ProductEvent outbox metrics
	pending, err := j.productEventsRepo.GetCount(ctx, false)
	if err != nil {
		slog.ErrorContext(ctx, "MetricCollectorJob: ProductEvents CountPending failed", "err", err)
	} else {
		j.metrics.SetPending("ProductEvent", pending)
	}

	deadLetters, err := j.productEventsRepo.GetCount(ctx, true)
	if err != nil {
		slog.ErrorContext(ctx, "MetricCollectorJob: ProductEvents CountDeadLetters failed", "err", err)
	} else {
		j.metrics.SetDeadLetter("ProductEvent", deadLetters)
	}

	// CacheUpdate outbox metrics
	cacheUpdatePending, err := j.cacheUpdateRepo.CountPending(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "MetricCollectorJob: CacheUpdate CountPending failed", "err", err)
	} else {
		j.metrics.SetPending("CacheUpdate", cacheUpdatePending)
	}

	cacheUpdateDeadLetters, err := j.cacheUpdateRepo.CountDeadLetters(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "MetricCollectorJob: CacheUpdate CountDeadLetters failed", "err", err)
	} else {
		j.metrics.SetDeadLetter("CacheUpdate", cacheUpdateDeadLetters)
	}

	// Pool metrics
	stat := j.pool.Stat()
	j.metrics.SetAcquiredConns(stat.AcquiredConns())
	j.metrics.SetIdleConns(stat.IdleConns())
	j.metrics.SetTotalConns(stat.TotalConns())
	j.metrics.SetMaxConns(stat.MaxConns())

	currentCount := stat.AcquireCount()
	currentDuration := stat.AcquireDuration()
	deltaCount := currentCount - j.prevAcquireCount
	if deltaCount > 0 {
		deltaDuration := currentDuration - j.prevAcquireDuration
		j.metrics.SetAvgAcquireDuration(deltaDuration / time.Duration(deltaCount))
	}
	j.prevAcquireCount = currentCount
	j.prevAcquireDuration = currentDuration
}
