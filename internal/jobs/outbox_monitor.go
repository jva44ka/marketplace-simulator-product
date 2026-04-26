package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
	enabled             bool
	interval            time.Duration
	prevAcquireCount    int64
	prevAcquireDuration time.Duration
}

func NewMetricCollectorJob(
	repo MetricCollectorRepository,
	pool *pgxpool.Pool,
	metrics MetricCollectorMetrics,
	enabled bool,
	interval time.Duration,
) *MetricCollectorJob {
	return &MetricCollectorJob{
		repo:     repo,
		pool:     pool,
		metrics:  metrics,
		enabled:  enabled,
		interval: interval,
	}
}

func (j *MetricCollectorJob) Run(ctx context.Context) {
	if !j.enabled {
		slog.InfoContext(ctx, "MetricCollectorJob disabled, shutting down")
		return
	}

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.tick(ctx)
		}
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
