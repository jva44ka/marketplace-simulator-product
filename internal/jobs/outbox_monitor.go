package jobs

import (
	"context"
	"log/slog"
	"time"
)

type OutboxMonitorRepository interface {
	GetCount(ctx context.Context, isDeadLetter bool) (int64, error)
}

type OutboxMonitorMetrics interface {
	SetPending(count int64)
	SetDeadLetter(count int64)
}

type OutboxMonitorJob struct {
	repo     OutboxMonitorRepository
	metrics  OutboxMonitorMetrics
	enabled  bool
	interval time.Duration
}

func NewOutboxMonitorJob(
	repo OutboxMonitorRepository,
	metrics OutboxMonitorMetrics,
	enabled bool,
	interval time.Duration,
) *OutboxMonitorJob {
	return &OutboxMonitorJob{
		repo:     repo,
		metrics:  metrics,
		enabled:  enabled,
		interval: interval,
	}
}

func (j *OutboxMonitorJob) Run(ctx context.Context) {
	if !j.enabled {
		slog.InfoContext(ctx, "OutboxMonitorJob disabled, shutting down")
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

func (j *OutboxMonitorJob) tick(ctx context.Context) {
	pending, err := j.repo.GetCount(ctx, false)
	if err != nil {
		slog.ErrorContext(ctx, "OutboxMonitorJob: CountPending failed", "err", err)
	} else {
		j.metrics.SetPending(pending)
	}

	deadLetters, err := j.repo.GetCount(ctx, true)
	if err != nil {
		slog.ErrorContext(ctx, "OutboxMonitorJob: CountDeadLetters failed", "err", err)
	} else {
		j.metrics.SetDeadLetter(deadLetters)
	}
}
