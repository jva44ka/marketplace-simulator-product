package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/models"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/kafka"
)

type ReservationRepository interface {
	GetExpired(ctx context.Context, cutoff time.Time) ([]models.Reservation, error)
}

type ProductService interface {
	ReleaseReservations(ctx context.Context, ids []int64) error
}

type ReservationExpiryJob struct {
	reservationRepo ReservationRepository
	productService  ProductService
	producer        *kafka.Producer
	reservationTTL  time.Duration
	interval        time.Duration
}

func NewReservationExpiryJob(
	reservationRepo ReservationRepository,
	productService ProductService,
	producer *kafka.Producer,
	reservationTTL time.Duration,
	interval time.Duration,
) *ReservationExpiryJob {
	return &ReservationExpiryJob{
		reservationRepo: reservationRepo,
		productService:  productService,
		producer:        producer,
		reservationTTL:  reservationTTL,
		interval:        interval,
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

	expired, err := j.reservationRepo.GetExpired(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("GetExpired: %w", err)
	}
	if len(expired) == 0 {
		return nil
	}

	ids := make([]int64, len(expired))
	for i, r := range expired {
		ids[i] = r.Id
	}

	if err = j.productService.ReleaseReservations(ctx, ids); err != nil {
		return fmt.Errorf("ReleaseReservations: %w", err)
	}

	for _, r := range expired {
		if pubErr := j.producer.PublishReservationExpired(ctx, r.Id, r.Sku, r.Count); pubErr != nil {
			slog.ErrorContext(ctx, "failed to publish reservation expired",
				"reservation_id", r.Id, "sku", r.Sku, "err", pubErr)
		}
	}

	return nil
}
