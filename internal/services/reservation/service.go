package reservation

import (
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/jva44ka/marketplace-simulator-product/internal/services"
)

type Service struct {
	transactor    services.Transactor
	products      services.ProductRepository
	reservations  services.ReservationRepository
	productOutbox services.ProductEventsOutboxRepository
	cacheOutbox   services.CacheUpdateOutboxRepository
}

func NewService(
	transactor services.Transactor,
	products services.ProductRepository,
	reservations services.ReservationRepository,
	productOutbox services.ProductEventsOutboxRepository,
	cacheOutbox services.CacheUpdateOutboxRepository,
) *Service {
	return &Service{
		transactor:    transactor,
		products:      products,
		reservations:  reservations,
		productOutbox: productOutbox,
		cacheOutbox:   cacheOutbox,
	}
}

func getProductMapSnapshot(productMap map[uint64]*models.Product) map[uint64]models.Product {
	snapshot := make(map[uint64]models.Product, len(productMap))
	for sku, p := range productMap {
		snapshot[sku] = *p
	}
	return snapshot
}
