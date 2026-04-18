package tracing

import (
	"context"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type PgxTracer struct {
	tracer trace.Tracer
}

func NewPgxTracer() *PgxTracer {
	return &PgxTracer{
		tracer: otel.Tracer("pgx"),
	}
}

func (t *PgxTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if !trace.SpanFromContext(ctx).SpanContext().IsValid() {
		return ctx
	}
	ctx, _ = t.tracer.Start(ctx, "db.query",
		trace.WithAttributes(
			attribute.String("db.statement", data.SQL),
			attribute.String("db.system", "postgresql"),
		),
	)
	return ctx
}

func (t *PgxTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span := trace.SpanFromContext(ctx)
	if data.Err != nil {
		span.RecordError(data.Err)
		span.SetStatus(codes.Error, data.Err.Error())
	}
	span.End()
}
