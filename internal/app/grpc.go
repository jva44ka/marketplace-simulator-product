package app

import (
	"context"
	"fmt"

	pb "github.com/jva44ka/marketplace-simulator-product/internal/app/pb/marketplace-simulator-product/api/v1/proto"
	"github.com/jva44ka/marketplace-simulator-product/internal/services/product"
	"github.com/jva44ka/marketplace-simulator-product/internal/services/reservation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ pb.ProductsServer = (*GrpcService)(nil)

type GrpcService struct {
	pb.UnimplementedProductsServer
	productService     *product.Service
	reservationService *reservation.Service
}

func NewGrpcService(svc *product.Service, resSvc *reservation.Service) *GrpcService {
	return &GrpcService{productService: svc, reservationService: resSvc}
}

func (s *GrpcService) GetProduct(ctx context.Context, request *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	if request.Sku < 1 {
		return nil, status.Errorf(codes.InvalidArgument, "sku must be more than zero")
	}

	p, err := s.productService.GetBySku(ctx, request.Sku)
	if err != nil {
		return nil, fmt.Errorf("GrpcService.GetProduct: %w", err)
	}

	return &pb.GetProductResponse{
		Sku:   p.Sku,
		Name:  p.Name,
		Price: p.Price,
		Count: p.Count - p.ReservedCount,
	}, nil
}

func (s *GrpcService) IncreaseProductCount(
	ctx context.Context,
	request *pb.IncreaseProductCountRequest) (*pb.IncreaseProductCountResponse, error) {
	if len(request.Products) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "products must not be empty")
	}

	seenSkus := make(map[uint64]struct{}, len(request.Products))
	products := make([]product.UpdateCount, 0, len(request.Products))
	for _, stock := range request.Products {
		if stock.Sku < 1 {
			return nil, status.Errorf(codes.InvalidArgument, "sku must be more than zero")
		}
		if _, exists := seenSkus[stock.Sku]; exists {
			return nil, status.Errorf(codes.InvalidArgument, "duplicate sku %d in request", stock.Sku)
		}
		seenSkus[stock.Sku] = struct{}{}
		products = append(products, product.UpdateCount{
			Sku:   stock.Sku,
			Delta: stock.Count,
		})
	}

	if err := s.productService.IncreaseCount(ctx, products); err != nil {
		return nil, fmt.Errorf("GrpcService.IncreaseProductCount: %w", err)
	}

	return &pb.IncreaseProductCountResponse{}, nil
}

func (s *GrpcService) ReserveProduct(
	ctx context.Context,
	request *pb.ReserveProductRequest) (*pb.ReserveProductResponse, error) {
	if len(request.Products) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "products must not be empty")
	}

	seenSkus := make(map[uint64]struct{}, len(request.Products))
	items := make([]reservation.ReserveItem, 0, len(request.Products))
	for _, stock := range request.Products {
		if stock.Sku < 1 {
			return nil, status.Errorf(codes.InvalidArgument, "sku must be more than zero")
		}
		if _, exists := seenSkus[stock.Sku]; exists {
			return nil, status.Errorf(codes.InvalidArgument, "duplicate sku %d in request", stock.Sku)
		}
		seenSkus[stock.Sku] = struct{}{}
		items = append(items, reservation.ReserveItem{
			Sku:   stock.Sku,
			Delta: stock.Count,
		})
	}

	reservationIds, err := s.reservationService.Reserve(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("GrpcService.ReserveProduct: %w", err)
	}

	results := make([]*pb.ReserveProductResponse_ReservationResult, 0, len(reservationIds))
	for sku, id := range reservationIds {
		results = append(results, &pb.ReserveProductResponse_ReservationResult{
			ReservationId: id,
			Sku:           sku,
		})
	}

	return &pb.ReserveProductResponse{Results: results}, nil
}

func (s *GrpcService) ReleaseReservation(
	ctx context.Context,
	request *pb.ReleaseReservationRequest) (*pb.ReleaseReservationResponse, error) {
	if len(request.ReservationIds) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "reservation_ids must not be empty")
	}

	if err := s.reservationService.Release(ctx, request.ReservationIds); err != nil {
		return nil, fmt.Errorf("GrpcService.ReleaseReservation: %w", err)
	}

	return &pb.ReleaseReservationResponse{}, nil
}

func (s *GrpcService) ConfirmReservation(
	ctx context.Context,
	request *pb.ConfirmReservationRequest) (*pb.ConfirmReservationResponse, error) {
	if len(request.ReservationIds) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "reservation_ids must not be empty")
	}

	if err := s.reservationService.Confirm(ctx, request.ReservationIds); err != nil {
		return nil, fmt.Errorf("GrpcService.ConfirmReservation: %w", err)
	}

	return &pb.ConfirmReservationResponse{}, nil
}
