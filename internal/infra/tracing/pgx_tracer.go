package tracing

import (
	"context"
	"fmt"

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

// ── QueryTracer ────────────────────────────────────────────────────────────────

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

// ── BatchTracer ────────────────────────────────────────────────────────────────

func (t *PgxTracer) TraceBatchStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceBatchStartData) context.Context {
	if !trace.SpanFromContext(ctx).SpanContext().IsValid() {
		return ctx
	}
	ctx, _ = t.tracer.Start(ctx, "db.batch",
		trace.WithAttributes(
			attribute.Int("db.batch.size", data.Batch.Len()),
			attribute.String("db.system", "postgresql"),
		),
	)
	return ctx
}

func (t *PgxTracer) TraceBatchQuery(ctx context.Context, _ *pgx.Conn, data pgx.TraceBatchQueryData) {
	span := trace.SpanFromContext(ctx)
	if data.Err != nil {
		span.RecordError(data.Err)
		span.SetStatus(codes.Error, data.Err.Error())
		return
	}
	// Каждый запрос в батче — отдельный event на родительском батч-спане
	span.AddEvent("db.batch.query", trace.WithAttributes(
		attribute.String("db.statement", data.SQL),
		attribute.String("db.rows_affected", fmt.Sprintf("%d", data.CommandTag.RowsAffected())),
	))
}

func (t *PgxTracer) TraceBatchEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceBatchEndData) {
	span := trace.SpanFromContext(ctx)
	if data.Err != nil {
		span.RecordError(data.Err)
		span.SetStatus(codes.Error, data.Err.Error())
	}
	span.End()
}
