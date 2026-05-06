package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type CacheClient[T any] struct {
	redisClient *redis.Client
	ttl         time.Duration
	tracer      trace.Tracer
}

func NewCacheClient[T any](client *redis.Client, ttl time.Duration) *CacheClient[T] {
	return &CacheClient[T]{
		redisClient: client,
		ttl:         ttl,
		tracer:      otel.Tracer("cache"),
	}
}

func (c *CacheClient[T]) Get(ctx context.Context, key string) (*T, error) {
	ctx, span, ok := c.startSpan(ctx, "cache.GET", "GET", key)
	if ok {
		defer span.End()
	}

	value, err := c.redisClient.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		if ok {
			span.SetAttributes(attribute.Bool("db.redis.cache_miss", true))
		}
		return nil, nil // cache miss — not an error
	}
	if err != nil {
		if ok {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return nil, err
	}

	var parsedValue T
	if err = json.Unmarshal(value, &parsedValue); err != nil {
		if ok {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return nil, err
	}
	return &parsedValue, nil
}

func (c *CacheClient[T]) Set(ctx context.Context, key string, value *T) error {
	ctx, span, ok := c.startSpan(ctx, "cache.SET", "SET", key)
	if ok {
		defer span.End()
	}

	data, err := json.Marshal(&value)
	if err != nil {
		if ok {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	}
	if err = c.redisClient.Set(ctx, key, data, c.ttl).Err(); err != nil {
		if ok {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	}

	return nil
}

func (c *CacheClient[T]) Delete(ctx context.Context, key string) error {
	ctx, span, ok := c.startSpan(ctx, "cache.DEL", "DEL", key)
	if ok {
		defer span.End()
	}

	if err := c.redisClient.Del(ctx, key).Err(); err != nil {
		if ok {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	}

	return nil
}

// startSpan creates a child span only when there is a valid parent in ctx
// (matching pgx_tracer behaviour — avoids orphan spans from background goroutines).
// Returns (ctx, span, true) when tracing is active, (ctx, noopSpan, false) otherwise.
func (c *CacheClient[T]) startSpan(ctx context.Context, spanName, operation, key string) (context.Context, trace.Span, bool) {
	if !trace.SpanFromContext(ctx).SpanContext().IsValid() {
		return ctx, nil, false
	}
	newCtx, span := c.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", operation),
			attribute.String("db.redis.key", key),
		),
	)
	return newCtx, span, true
}
