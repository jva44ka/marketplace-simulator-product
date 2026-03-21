package app

import (
	"context"
	"errors"

	pb "github.com/jva44ka/ozon-simulator-go-products/internal/app/gen/ozon-simulator-go-products/api/v1/proto"
	errors2 "github.com/jva44ka/ozon-simulator-go-products/internal/domain/errors"
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
		return nil, grpcStatusFromErr(err)
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
		return nil, grpcStatusFromErr(err)
	}

	return &pb.IncreaseProductCountResponse{}, nil
}

func (s *GrpcService) DecreaseProductCount(
	ctx context.Context,
	request *pb.DecreaseProductCountRequest) (*pb.DecreaseProductCountResponse, error) {
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

	if err := s.ProductService.DecreaseCount(ctx, products); err != nil {
		return nil, grpcStatusFromErr(err)
	}

	return &pb.DecreaseProductCountResponse{}, nil
}

// grpcStatusFromErr маппит доменные ошибки на gRPC status codes.
// Неизвестные ошибки → codes.Internal.
func grpcStatusFromErr(err error) error {
	switch {
	case errors.Is(err, errors2.ErrProductNotFound):
		return status.Errorf(codes.NotFound, err.Error())
	case errors.Is(err, errors2.ErrInsufficientProduct):
		return status.Errorf(codes.FailedPrecondition, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}
