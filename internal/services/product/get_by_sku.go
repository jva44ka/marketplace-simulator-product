package product

import (
	"context"
	"fmt"

	"github.com/jva44ka/ozon-simulator-go-products/internal/errors"
	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
)

func (s *Service) GetBySku(ctx context.Context, sku uint64) (*models.Product, error) {
	product, err := s.db.Products().GetBySku(ctx, sku)
	if err != nil {
		return nil, fmt.Errorf("productRepository.GetBySku: %w", err)
	}
	if product == nil {
		return nil, errors.NewProductNotFoundError(sku)
	}

	if product.ReservedCount > 0 {
		if product.Count >= product.ReservedCount {
			//TODO: Возможно стоит оставлять в отдельном поле ReservedCount, а не смешивать 2 поля
			product.Count -= product.ReservedCount
			//TODO: резерваций меньше чем оставшихся продуктов
		} else {
			product.Count = 0
			//TODO: резерваций больше чем оставшихся продуктов, алертим
		}
	}

	return product, nil
}
