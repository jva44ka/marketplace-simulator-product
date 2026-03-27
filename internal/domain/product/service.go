package product

import (
	"context"
	"fmt"
	"maps"
	"slices"

	domainErrors "github.com/jva44ka/ozon-simulator-go-products/internal/domain/errors"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/models"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/reservation"
)

type ProductRepository interface {
	GetProductBySku(ctx context.Context, sku uint64) (*models.Product, error)
	GetProductsBySkus(ctx context.Context, skus []uint64) ([]*models.Product, error)
	UpdateCount(ctx context.Context, products []*models.Product) error
}

type ReservationRepository interface {
	Insert(ctx context.Context, sku uint64, count uint32) (reservation.Reservation, error)
	GetByIds(ctx context.Context, ids []int64) ([]reservation.Reservation, error)
	DeleteByIds(ctx context.Context, ids []int64) error
}

type Transactor interface {
	InTransaction(ctx context.Context, fn func(repos Repositories) error) error
}

type Service struct {
	productRepository     ProductRepository
	reservationRepository ReservationRepository
	transactor            Transactor
}

func NewService(productRepository ProductRepository, reservationRepository ReservationRepository, transactor Transactor) *Service {
	return &Service{
		productRepository:     productRepository,
		reservationRepository: reservationRepository,
		transactor:            transactor,
	}
}

type UpdateCount struct {
	Sku   uint64
	Delta uint32
}

func (s *Service) GetProductBySku(ctx context.Context, sku uint64) (*models.Product, error) {
	product, err := s.productRepository.GetProductBySku(ctx, sku)
	if err != nil {
		return nil, fmt.Errorf("productRepository.GetProductBySku: %w", err)
	}

	if product == nil {
		return nil, domainErrors.NewProductNotFoundError(sku)
	}

	return product, nil
}

func (s *Service) IncreaseCount(ctx context.Context, products []UpdateCount) error {
	existingProductsMap, err := validateProductsExist(ctx, products, s.productRepository)
	if err != nil {
		return err
	}
	for _, product := range products {
		existingProductsMap[product.Sku].Count += product.Delta
	}
	return s.productRepository.UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap)))
}

func (s *Service) Reserve(ctx context.Context, products []UpdateCount) (map[uint64]int64, error) {
	reservationIds := make(map[uint64]int64, len(products))
	err := s.transactor.InTransaction(ctx, func(repos Repositories) error {
		existingProductsMap, err := validateProductsExist(ctx, products, repos.Products)
		if err != nil {
			return err
		}
		for _, product := range products {
			existingProduct := existingProductsMap[product.Sku]
			if existingProduct.Count < product.Delta {
				return domainErrors.NewInsufficientProductError(product.Sku, existingProduct.Count, product.Delta)
			}
			existingProduct.Count -= product.Delta
		}
		if err = repos.Products.UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap))); err != nil {
			return fmt.Errorf("ProductService.Reserve: %w", err)
		}
		for _, p := range products {
			rv, err := repos.Reservations.Insert(ctx, p.Sku, p.Delta)
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
	return s.transactor.InTransaction(ctx, func(repos Repositories) error {
		reservations, err := repos.Reservations.GetByIds(ctx, ids)
		if err != nil {
			return fmt.Errorf("ProductService.ReleaseReservations: %w", err)
		}
		products := make([]UpdateCount, len(reservations))
		for i, r := range reservations {
			products[i] = UpdateCount{Sku: r.Sku, Delta: r.Count}
		}
		existingProductsMap, err := validateProductsExist(ctx, products, repos.Products)
		if err != nil {
			return err
		}
		for _, p := range products {
			existingProductsMap[p.Sku].Count += p.Delta
		}
		if err = repos.Products.UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap))); err != nil {
			return err
		}
		return repos.Reservations.DeleteByIds(ctx, ids)
	})
}

func (s *Service) ConfirmReservations(ctx context.Context, ids []int64) error {
	return s.reservationRepository.DeleteByIds(ctx, ids)
}

// ReleaseReservation используется фоновой джобой — без транзакции,
// только обновление счётчика товаров.
func (s *Service) ReleaseReservation(ctx context.Context, products []UpdateCount) error {
	existingProductsMap, err := validateProductsExist(ctx, products, s.productRepository)
	if err != nil {
		return err
	}
	for _, p := range products {
		existingProductsMap[p.Sku].Count += p.Delta
	}
	return s.productRepository.UpdateCount(ctx, slices.Collect(maps.Values(existingProductsMap)))
}

func validateProductsExist(ctx context.Context, products []UpdateCount, repo ProductRepository) (map[uint64]*models.Product, error) {
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
