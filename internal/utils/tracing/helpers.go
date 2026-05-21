package tracing

import (
	"context"

	"github.com/naukograd-software/komeet-backend/internal/core/logger"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"
)

func GetTraceLogger(name string) *otelzap.Logger {
	l := logger.Global().Named(name)
	return otelzap.New(
		l,
		otelzap.WithCaller(l.Level() == zap.DebugLevel),
		otelzap.WithStackTrace(l.Level() == zap.DebugLevel),
	)
}

func handelErr(span trace.Span, err error, opts *traceOptions) error {
	if err != nil && opts.errorFilter(err) {
		//span.SetAttributes(attribute.Bool("error", true))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
	return err
}

type TraceAbleFunc func(spanCtx context.Context, span trace.Span) error

type TraceAbleRetFunc[T any] func(spanCtx context.Context, span trace.Span) (T, error)

type TraceAbleRet2Func[T any, U any] func(spanCtx context.Context, span trace.Span) (T, U, error)

type traceOptions struct {
	startSpanOpts []trace.SpanStartOption
	handlePanic   bool
	errorFilter   func(err error) bool
}

func newOpts(opts ...TraceOption) *traceOptions {
	o := &traceOptions{
		startSpanOpts: make([]trace.SpanStartOption, 0),
		handlePanic:   true,
		errorFilter: func(err error) bool {
			return true
		},
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

type TraceOption func(opts *traceOptions)

func WithSpanOpts(options ...trace.SpanStartOption) TraceOption {
	return func(opts *traceOptions) {
		opts.startSpanOpts = options
	}
}

func WithErrFilter(errFiter func(err error) bool) TraceOption {
	return func(opts *traceOptions) {
		opts.errorFilter = errFiter
	}
}

func WithAttrs(attrs ...attribute.KeyValue) TraceOption {
	return func(opts *traceOptions) {
		opts.startSpanOpts = append(
			opts.startSpanOpts,
			trace.WithAttributes(attrs...),
		)
	}
}

func WithNoHandlePanic() TraceOption {
	return func(opts *traceOptions) {
		opts.handlePanic = false
	}
}

func WithTraceErr(tracer trace.Tracer, ctx context.Context, name string, f TraceAbleFunc, opts ...TraceOption) error {
	o := newOpts(opts...)
	var call func(spanCtx context.Context, span trace.Span) error
	if o.handlePanic {
		call = func(spanCtx context.Context, span trace.Span) error {
			return f(spanCtx, span)
		}
	} else {
		call = f
	}
	newCtx, span := tracer.Start(ctx, name, o.startSpanOpts...)
	return handelErr(span, call(newCtx, span), o)
}

func WithTraceRetErr[T any](tracer trace.Tracer, ctx context.Context, name string, f TraceAbleRetFunc[T], opts ...TraceOption) (T, error) {
	o := newOpts(opts...)
	newCtx, span := tracer.Start(ctx, name, o.startSpanOpts...)
	var call func(spanCtx context.Context, span trace.Span) (T, error)
	if o.handlePanic {
		call = func(spanCtx context.Context, span trace.Span) (T, error) {
			return f(spanCtx, span)
		}
	} else {
		call = f
	}
	ret, err := call(newCtx, span)
	return ret, handelErr(span, err, o)
}

func WithTraceRet2Err[T any, U any](tracer trace.Tracer, ctx context.Context, name string, f TraceAbleRet2Func[T, U], opts ...TraceOption) (T, U, error) {
	o := newOpts(opts...)
	newCtx, span := tracer.Start(ctx, name, o.startSpanOpts...)
	var call func(spanCtx context.Context, span trace.Span) (T, U, error)
	if o.handlePanic {
		call = func(spanCtx context.Context, span trace.Span) (T, U, error) {
			return f(spanCtx, span)
		}
	} else {
		call = f
	}
	ret1, ret2, err := call(newCtx, span)
	return ret1, ret2, handelErr(span, err, o)
}

func WithTraceProtoEmpty(tracer trace.Tracer, ctx context.Context, name string, f TraceAbleFunc, opts ...TraceOption) (*emptypb.Empty, error) {
	err := WithTraceErr(tracer, ctx, name, f, opts...)
	return &emptypb.Empty{}, err
}
