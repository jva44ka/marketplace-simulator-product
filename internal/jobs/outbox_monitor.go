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

type MetricCollectorMetrics interface {
	SetPending(count int64)
	SetDeadLetter(count int64)
	SetAcquiredConns(n int32)
	SetIdleConns(n int32)
	SetTotalConns(n int32)
	SetMaxConns(n int32)
	SetAvgAcquireDuration(d time.Duration)
}

type MetricCollectorJob struct {
	repo                MetricCollectorRepository
	pool                *pgxpool.Pool
	metrics             MetricCollectorMetrics
	cfgStore            *config.ConfigStore
	prevAcquireCount    int64
	prevAcquireDuration time.Duration
}

func NewMetricCollectorJob(
	repo MetricCollectorRepository,
	pool *pgxpool.Pool,
	metrics MetricCollectorMetrics,
	cfgStore *config.ConfigStore,
) *MetricCollectorJob {
	return &MetricCollectorJob{
		repo:     repo,
		pool:     pool,
		metrics:  metrics,
		cfgStore: cfgStore,
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
	// Outbox metrics
	pending, err := j.repo.GetCount(ctx, false)
	if err != nil {
		slog.ErrorContext(ctx, "MetricCollectorJob: CountPending failed", "err", err)
	} else {
		j.metrics.SetPending(pending)
	}

	deadLetters, err := j.repo.GetCount(ctx, true)
	if err != nil {
		slog.ErrorContext(ctx, "MetricCollectorJob: CountDeadLetters failed", "err", err)
	} else {
		j.metrics.SetDeadLetter(deadLetters)
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
