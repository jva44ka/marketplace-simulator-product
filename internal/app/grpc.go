package app

import (
	"context"
	"fmt"

	pb "github.com/jva44ka/ozon-simulator-go-products/internal/app/gen/ozon-simulator-go-products/api/v1/proto"
	"github.com/jva44ka/ozon-simulator-go-products/internal/domain/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ pb.ProductsServer = (*GrpcService)(nil)

type GrpcService struct {
	pb.UnimplementedProductsServer
	ProductService service.ProductService
}

func NewGrpcService(productService *service.ProductService) *GrpcService {
	return &GrpcService{ProductService: *productService}
}

func (s *GrpcService) GetProduct(ctx context.Context, request *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	if request.Sku < 1 {
		return nil, status.Errorf(codes.InvalidArgument, "sku must be more than zero")
	}

	product, err := s.ProductService.GetProductBySku(ctx, request.Sku)
	if err != nil {
		return nil, fmt.Errorf("GrpcService.GetProduct: %w", err)
	}

	return &pb.GetProductResponse{
		Sku:   product.Sku,
		Name:  product.Name,
		Price: product.Price,
		Count: product.Count,
	}, nil
}

func (s *GrpcService) IncreaseProductCount(
	ctx context.Context,
	request *pb.IncreaseProductCountRequest) (*pb.IncreaseProductCountResponse, error) {
	if len(request.Products) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "products must not be empty")
	}

	products := make([]service.UpdateProductCount, 0, len(request.Products))
	for _, stock := range request.Products {
		if stock.Sku < 1 {
			return nil, status.Errorf(codes.InvalidArgument, "sku must be more than zero")
		}
		products = append(products, service.UpdateProductCount{
			Sku:   stock.Sku,
			Delta: stock.Count,
		})
	}

	if err := s.ProductService.IncreaseCount(ctx, products); err != nil {
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

	products := make([]service.UpdateProductCount, 0, len(request.Products))
	for _, stock := range request.Products {
		if stock.Sku < 1 {
			return nil, status.Errorf(codes.InvalidArgument, "sku must be more than zero")
		}
		products = append(products, service.UpdateProductCount{
			Sku:   stock.Sku,
			Delta: stock.Count,
		})
	}

	reservationIds, err := s.ProductService.ReserveCount(ctx, products)
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

	if err := s.ProductService.ReleaseReservationByIds(ctx, request.ReservationIds); err != nil {
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

	if err := s.ProductService.ConfirmReservationByIds(ctx, request.ReservationIds); err != nil {
		return nil, fmt.Errorf("GrpcService.ConfirmReservation: %w", err)
	}

	return &pb.ConfirmReservationResponse{}, nil
}
