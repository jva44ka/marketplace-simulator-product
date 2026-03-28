package services

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain"
	domainErrors "github.com/jva44ka/ozon-simulator-go-products/internal/domain/errors"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/models"
)

type Service struct {
	db domain.DBManager
}

func NewService(db domain.DBManager) *Service {
	return &Service{db: db}
}

type UpdateCount struct {
	Sku   uint64
	Delta uint32
}

func (s *Service) GetProductBySku(ctx context.Context, sku uint64) (*models.Product, error) {
	product, err := s.db.Products().GetProductBySku(ctx, sku)
	if err != nil {
		return nil, fmt.Errorf("productRepository.GetProductBySku: %w", err)
	}
	if product == nil {
		return nil, domainErrors.NewProductNotFoundError(sku)
	}
	return product, nil
}

func (s *Service) IncreaseCount(ctx context.Context, products []UpdateCount) error {
	existingProductsMap, err := validateProductsExist(ctx, products, s.db.Products())
	if err != nil {
		return err
	}
	for _, p := range products {
		existingProductsMap[p.Sku].Count += p.Delta
	}
	return s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		return s.db.Products().WithTx(tx).UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap)))
	})
}

func (s *Service) Reserve(ctx context.Context, products []UpdateCount) (map[uint64]int64, error) {
	existingProductsMap, err := validateProductsExist(ctx, products, s.db.Products())
	if err != nil {
		return nil, err
	}
	for _, product := range products {
		existingProduct := existingProductsMap[product.Sku]
		if existingProduct.Count < product.Delta {
			return nil, domainErrors.NewInsufficientProductError(product.Sku, existingProduct.Count, product.Delta)
		}
		existingProduct.Count -= product.Delta
	}

	reservationIds := make(map[uint64]int64, len(products))
	err = s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		if err := s.db.Products().WithTx(tx).UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap))); err != nil {
			return fmt.Errorf("ProductService.Reserve: %w", err)
		}
		for _, p := range products {
			rv, err := s.db.Reservations().WithTx(tx).Insert(ctx, p.Sku, p.Delta)
			if err != nil {
				return fmt.Errorf("ProductService.Reserve: %w", err)
			}
			reservationIds[p.Sku] = rv.Id
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return reservationIds, nil
}

func (s *Service) ReleaseReservations(ctx context.Context, ids []int64) error {
	reservations, err := s.db.Reservations().GetByIds(ctx, ids)
	if err != nil {
		return fmt.Errorf("ProductService.ReleaseReservations: %w", err)
	}
	products := make([]UpdateCount, len(reservations))
	for i, r := range reservations {
		products[i] = UpdateCount{Sku: r.Sku, Delta: r.Count}
	}
	existingProductsMap, err := validateProductsExist(ctx, products, s.db.Products())
	if err != nil {
		return err
	}
	for _, p := range products {
		existingProductsMap[p.Sku].Count += p.Delta
	}

	return s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		if err := s.db.Products().WithTx(tx).UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap))); err != nil {
			return err
		}
		return s.db.Reservations().WithTx(tx).DeleteByIds(ctx, ids)
	})
}

func (s *Service) ConfirmReservations(ctx context.Context, ids []int64) error {
	return s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		return s.db.Reservations().WithTx(tx).DeleteByIds(ctx, ids)
	})
}

func (s *Service) ReleaseReservation(ctx context.Context, products []UpdateCount) error {
	existingProductsMap, err := validateProductsExist(ctx, products, s.db.Products())
	if err != nil {
		return err
	}
	for _, p := range products {
		existingProductsMap[p.Sku].Count += p.Delta
	}
	return s.db.InTransaction(ctx, func(tx pgx.Tx) error {
		return s.db.Products().WithTx(tx).UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap)))
	})
}

func validateProductsExist(
	ctx context.Context,
	products []UpdateCount,
	repo domain.ProductReadRepository) (map[uint64]*models.Product, error) {
	skus := make([]uint64, 0, len(products))
	for _, product := range products {
		skus = append(skus, product.Sku)
	}

	existingProducts, err := repo.GetProductsBySkus(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("ProductService.validateProductsExist: %w", err)
	}

	existingProductsMap := make(map[uint64]*models.Product, len(existingProducts))
	for _, existingProduct := range existingProducts {
		existingProductsMap[existingProduct.Sku] = existingProduct
	}

	for _, product := range products {
		if _, ok := existingProductsMap[product.Sku]; !ok {
			return nil, domainErrors.NewProductNotFoundError(product.Sku)
		}
	}

	return existingProductsMap, nil
}
