package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/reservation"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/service"
	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/kafka"
)

type ReservationRepository interface {
	GetExpired(ctx context.Context) ([]reservation.Reservation, error)
	DeleteByIds(ctx context.Context, ids []int64) error
}

type ProductService interface {
	ReleaseReservation(ctx context.Context, products []service.UpdateProductCount) error
}

type ReservationExpiryJob struct {
	reservationRepo ReservationRepository
	productService  ProductService
	producer        *kafka.Producer
	interval        time.Duration
}

func NewReservationExpiryJob(
	reservationRepo ReservationRepository,
	productService ProductService,
	producer *kafka.Producer,
	interval time.Duration,
) *ReservationExpiryJob {
	return &ReservationExpiryJob{
		reservationRepo: reservationRepo,
		productService:  productService,
		producer:        producer,
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
	expired, err := j.reservationRepo.GetExpired(ctx)
	if err != nil {
		return fmt.Errorf("GetExpired: %w", err)
	}
	if len(expired) == 0 {
		return nil
	}

	// Aggregate counts by SKU for the release call
	grouped := make(map[uint64]uint32, len(expired))
	for _, r := range expired {
		grouped[r.Sku] += r.Count
	}

	products := make([]service.UpdateProductCount, 0, len(grouped))
	for sku, count := range grouped {
		products = append(products, service.UpdateProductCount{Sku: sku, Delta: count})
	}

	if err = j.productService.ReleaseReservation(ctx, products); err != nil {
		return fmt.Errorf("ReleaseReservation: %w", err)
	}

	ids := make([]int64, len(expired))
	for i, r := range expired {
		ids[i] = r.Id
	}

	if err = j.reservationRepo.DeleteByIds(ctx, ids); err != nil {
		return fmt.Errorf("DeleteByIds: %w", err)
	}

	for _, r := range expired {
		if pubErr := j.producer.PublishReservationExpired(ctx, r.Id, r.Sku, r.Count); pubErr != nil {
			slog.ErrorContext(ctx, "failed to publish reservation expired",
				"reservation_id", r.Id, "sku", r.Sku, "err", pubErr)
		}
	}

	return nil
}
