package tracing

import (
	"context"

	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/stroppy-io/stroppy-cloud/internal/core/logger"
)

// Entity is an embeddable struct that gives a type its own tracer + otelzap logger.
type Entity struct {
	tracer trace.Tracer
	logger *otelzap.Logger
}

func NewEntity(name string) *Entity {
	return &Entity{tracer: otel.Tracer(name), logger: newLogger(name)}
}

func newLogger(name string) *otelzap.Logger {
	return otelzap.New(logger.Global().Named(name))
}

func (e *Entity) Tracer() trace.Tracer    { return e.tracer }
func (e *Entity) Logger() *otelzap.Logger { return e.logger }

func (e *Entity) Trace(ctx context.Context, name string, fn func(ctx context.Context, span trace.Span) error) error {
	return WithTraceErr(e.tracer, ctx, name, fn)
}
