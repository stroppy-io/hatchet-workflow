package tracing

import (
	"context"

	"github.com/gopherex/xlog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type Entity struct {
	tracer trace.Tracer
	logger *xlog.Logger
}

func (e *Entity) Logger() *xlog.Logger {
	return e.logger
}

func (e *Entity) Tracer() trace.Tracer {
	return e.tracer
}

func (e *Entity) Trace(ctx context.Context, name string, f TraceAbleFunc, opts ...TraceOption) error {
	return WithTraceErr(e.tracer, ctx, name, f, opts...)
}

func NewEntity(logger *xlog.Logger) *Entity {
	return &Entity{
		tracer: otel.Tracer(logger.Name()),
		logger: logger,
	}
}
