package middleware

import (
	"context"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func RateLimit(limiter *rate.Limiter) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if !limiter.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "api rate limit exceeded")
		}
		return handler(ctx, req)
	}
}
