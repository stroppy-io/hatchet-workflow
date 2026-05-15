package tracing

import (
	"context"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func WithTraceErr(tracer trace.Tracer, ctx context.Context, name string, fn func(ctx context.Context, span trace.Span) error) error {
	ctx, span := tracer.Start(ctx, name)
	defer span.End()
	err := fn(ctx, span)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

func WithTraceRet[T any](tracer trace.Tracer, ctx context.Context, name string, fn func(ctx context.Context, span trace.Span) (T, error)) (T, error) {
	ctx, span := tracer.Start(ctx, name)
	defer span.End()
	v, err := fn(ctx, span)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}
