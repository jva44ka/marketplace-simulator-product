package middleware

import (
	"context"
	"errors"

	domainErrors "github.com/jva44ka/ozon-simulator-go-products/internal/domain/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func StatusCode(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	resp, err = handler(ctx, req)

	return resp, grpcStatusFromErr(err)
}

func grpcStatusFromErr(err error) error {
	switch {
	case errors.Is(err, &domainErrors.ProductNotFoundError{}):
		return status.Errorf(codes.NotFound, err.Error())
	case errors.Is(err, &domainErrors.InsufficientProductError{}):
		return status.Errorf(codes.FailedPrecondition, err.Error())
	default:
		return status.Errorf(codes.Internal, err.Error())
	}
}
