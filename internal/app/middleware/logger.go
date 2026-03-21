package middleware

import (
	"context"
	"log/slog"

	"github.com/jva44ka/ozon-simulator-go-products/internal/infra/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func Logger(cfg *config.Config) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if cfg.Logging.LogRequestBody {
			body, _ := protojson.Marshal((req).(proto.Message))
			slog.InfoContext(ctx, "request", "method", info.FullMethod, "body", string(body))
		}

		if resp, err = handler(ctx, req); err != nil {
			logError(ctx, info.FullMethod, err)
		}

		if cfg.Logging.LogResponseBody {
			if v, ok := resp.(proto.Message); ok {
				body, _ := protojson.Marshal(v)
				slog.InfoContext(ctx, "response", "method", info.FullMethod, "body", string(body))
			}
		}

		return resp, err
	}
}

func logError(ctx context.Context, method string, err error) {
	st, ok := status.FromError(err)
	if !ok {
		slog.ErrorContext(ctx, "request failed", "method", method, "err", err)
		return
	}

	if isClientError(st.Code()) {
		slog.WarnContext(ctx, "request failed", "method", method, "code", st.Code(), "err", st.Message())
	} else {
		slog.ErrorContext(ctx, "request failed", "method", method, "code", st.Code(), "err", st.Message())
	}
}

func isClientError(code codes.Code) bool {
	switch code {
	case codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.FailedPrecondition,
		codes.OutOfRange,
		codes.Canceled:
		return true
	default:
		return false
	}
}
