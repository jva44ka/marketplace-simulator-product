package redis

import "github.com/jva44ka/marketplace-simulator-product/internal/models"

func FromModel(model *models.Product) *Product {
	return &Product{
		Sku:           model.Sku,
		Name:          model.Name,
		Price:         model.Price,
		Count:         model.Count,
		ReservedCount: model.ReservedCount,
		TransactionId: model.TransactionId,
	}
}

type Product struct {
	Sku           uint64  `json:"sku"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	Count         uint32  `json:"count"`
	ReservedCount uint32  `json:"reserved_count"`
	TransactionId uint32  `json:"transaction_id"`
}
