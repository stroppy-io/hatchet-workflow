package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type Entity struct {
	tracer trace.Tracer
}

func (e *Entity) Tracer() trace.Tracer {
	return e.tracer
}

func (e *Entity) Trace(ctx context.Context, name string, f TraceAbleFunc, opts ...TraceOption) error {
	return WithTraceErr(e.tracer, ctx, name, f, opts...)
}

func NewEntity(name string) *Entity {
	return &Entity{
		tracer: otel.Tracer(name),
	}
}
