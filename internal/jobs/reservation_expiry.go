package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jva44ka/marketplace-simulator-product/internal/infra/config"
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

type ReservationRepository interface {
	GetExpired(ctx context.Context, cutoff time.Time) ([]models.Reservation, error)
}

type ReservationService interface {
	Release(ctx context.Context, ids []int64) error
}

type ReservationExpiryJob struct {
	reservationRepo    ReservationRepository
	reservationService ReservationService
	cfgStore           *config.ConfigStore
}

func NewReservationExpiryJob(
	reservationRepo ReservationRepository,
	reservationSvc ReservationService,
	cfgStore *config.ConfigStore,
) *ReservationExpiryJob {
	return &ReservationExpiryJob{
		reservationRepo:    reservationRepo,
		reservationService: reservationSvc,
		cfgStore:           cfgStore,
	}
}

func (j *ReservationExpiryJob) Run(ctx context.Context) {
	for {
		cfg := j.cfgStore.Load().Jobs.ReservationExpiry

		//todo рефакторинг, чтобы не парсить на каждую итерацию
		interval, err := time.ParseDuration(cfg.JobInterval)
		if err != nil {
			interval = time.Second
			slog.Warn("ReservationExpiryJob: invalid job-interval, using 1s", "err", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		if !cfg.Enabled {
			continue
		}

		ttl, err := time.ParseDuration(cfg.TTL)
		if err != nil {
			slog.Warn("ReservationExpiryJob: invalid ttl, skipping tick", "err", err)
			continue
		}

		if err := j.tick(ctx, ttl); err != nil {
			slog.ErrorContext(ctx, "reservation expiry job failed", "err", err)
		}
	}
}

func (j *ReservationExpiryJob) tick(ctx context.Context, ttl time.Duration) error {
	cutoff := time.Now().Add(-ttl)

	//TODO: сделать все через сервис
	expiredReservations, err := j.reservationRepo.GetExpired(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("GetExpired: %w", err)
	}
	if len(expiredReservations) == 0 {
		return nil
	}

	ids := make([]int64, len(expiredReservations))
	for i, r := range expiredReservations {
		ids[i] = r.Id
	}

	return j.reservationService.Release(ctx, ids)
}
