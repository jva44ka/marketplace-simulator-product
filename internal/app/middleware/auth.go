package middleware

import (
	"context"

	"github.com/jva44ka/marketplace-simulator-product/internal/infra/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func Auth(store *config.ConfigStore) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		cfg := store.Load()
		if !cfg.Authorization.Enabled {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "no metadata")
		}

		values := md.Get("x-auth")

		for _, value := range values {
			if value == cfg.Authorization.AdminUser {
				return handler(ctx, req)
			}
		}

		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
}
