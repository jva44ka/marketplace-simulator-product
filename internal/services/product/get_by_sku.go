package product

import (
	"context"
	"fmt"

	"github.com/jva44ka/ozon-simulator-go-products/internal/errors"
	"github.com/jva44ka/ozon-simulator-go-products/internal/models"
)

func (s *Service) GetBySku(ctx context.Context, sku uint64) (*models.Product, error) {
	product, err := s.db.Products().GetProductBySku(ctx, sku)
	if err != nil {
		return nil, fmt.Errorf("productRepository.GetBySku: %w", err)
	}
	if product == nil {
		return nil, errors.NewProductNotFoundError(sku)
	}
	return product, nil
}
