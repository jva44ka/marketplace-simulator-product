package reservation

import (
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
)

type Service struct {
	transactor      Transactor
	productRepo     ReadProductRepository
	reservationRepo ReadReservationRepository
}

func NewService(
	transactor Transactor,
	products ReadProductRepository,
	reservations ReadReservationRepository,
) *Service {
	return &Service{
		transactor:      transactor,
		productRepo:     products,
		reservationRepo: reservations,
	}
}

func getProductMapSnapshot(productMap map[uint64]*models.Product) map[uint64]models.Product {
	snapshot := make(map[uint64]models.Product, len(productMap))
	for sku, p := range productMap {
		snapshot[sku] = *p
	}
	return snapshot
}
