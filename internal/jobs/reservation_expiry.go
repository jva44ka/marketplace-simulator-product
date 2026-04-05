package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
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
	reservationTTL     time.Duration
	interval           time.Duration
}

func NewReservationExpiryJob(
	reservationRepo ReservationRepository,
	reservationSvc ReservationService,
	reservationTTL time.Duration,
	interval time.Duration,
) *ReservationExpiryJob {
	return &ReservationExpiryJob{
		reservationRepo:    reservationRepo,
		reservationService: reservationSvc,
		reservationTTL:     reservationTTL,
		interval:           interval,
	}
}

func (j *ReservationExpiryJob) Run(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := j.tick(ctx); err != nil {
				slog.ErrorContext(ctx, "reservation expiry job failed", "err", err)
			}
		}
	}
}

func (j *ReservationExpiryJob) tick(ctx context.Context) error {
	cutoff := time.Now().Add(-j.reservationTTL)

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
