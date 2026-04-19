package reservation

import (
	"github.com/jva44ka/marketplace-simulator-product/internal/models"
	"github.com/jva44ka/marketplace-simulator-product/internal/services"
)

type Service struct {
	db services.DBManager
}

func NewService(db services.DBManager) *Service {
	return &Service{db: db}
}

func getProductMapSnapshot(productMap map[uint64]*models.Product) map[uint64]models.Product {
	snapshot := make(map[uint64]models.Product, len(productMap))

	for sku, productMapItem := range productMap {
		snapshot[sku] = *productMapItem
	}

	return snapshot
}
