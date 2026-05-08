package reservation

import "github.com/jva44ka/marketplace-simulator-product/internal/models"

func getProductMapSnapshot(productMap map[uint64]*models.Product) map[uint64]models.Product {
	snapshot := make(map[uint64]models.Product, len(productMap))
	for sku, p := range productMap {
		snapshot[sku] = *p
	}
	return snapshot
}
