// Package tracing OpenTelemetry env config (see OTel SDK/OTLP docs):
// OTEL_SERVICE_NAME=...                      # Service name (if not set, SDK may derive or use defaults)
// OTEL_RESOURCE_ATTRIBUTES=key=val,...       # Extra resource attributes (comma-separated key=value list)
//
// OTEL_TRACES_EXPORTER=otlp|jaeger|zipkin|console|none   # Traces exporter(s)
// OTEL_METRICS_EXPORTER=otlp|prometheus|console|none     # Metrics exporter(s)
// OTEL_LOGS_EXPORTER=otlp|console|none                   # Logs exporter(s)
//
// OTEL_EXPORTER_OTLP_ENDPOINT=http://host:4318           # Base OTLP endpoint
// OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=...                 # Traces-only endpoint
// OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=...                # Metrics-only endpoint
// OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=...                   # Logs-only endpoint
//
// OTEL_EXPORTER_OTLP_PROTOCOL=grpc|http/protobuf         # OTLP protocol (global)
// OTEL_EXPORTER_OTLP_TRACES_PROTOCOL=grpc|http/protobuf  # Traces protocol override
// OTEL_EXPORTER_OTLP_METRICS_PROTOCOL=grpc|http/protobuf # Metrics protocol override
// OTEL_EXPORTER_OTLP_LOGS_PROTOCOL=grpc|http/protobuf    # Logs protocol override
//
// OTEL_EXPORTER_OTLP_HEADERS=k=v,k2=v2                   # Extra headers for OTLP requests
// OTEL_EXPORTER_OTLP_TRACES_HEADERS=...                  # Traces headers override
// OTEL_EXPORTER_OTLP_METRICS_HEADERS=...                 # Metrics headers override
// OTEL_EXPORTER_OTLP_LOGS_HEADERS=...                    # Logs headers override
//
// OTEL_EXPORTER_OTLP_TIMEOUT=10000                       # Export timeout (ms)
// OTEL_EXPORTER_OTLP_TRACES_TIMEOUT=...                  # Traces timeout override
// OTEL_EXPORTER_OTLP_METRICS_TIMEOUT=...                 # Metrics timeout override
// OTEL_EXPORTER_OTLP_LOGS_TIMEOUT=...                    # Logs timeout override
//
// OTEL_TRACES_SAMPLER=parentbased_traceidratio|always_on|always_off|traceidratio # Sampler
// OTEL_TRACES_SAMPLER_ARG=0.1                            # Sampler argument (e.g., ratio)
//
// OTEL_PROPAGATORS=tracecontext,baggage,b3,...           # Context propagators (comma-separated)
package tracing

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/naukograd-software/komeet-backend/internal/core/build"
	"go.opentelemetry.io/contrib/instrumentation/host"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	logsExport "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	metricExport "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	traceExport "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	logGlobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

func newResource() (*resource.Resource, error) {
	hostName, _ := os.Hostname()
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(build.ServiceName),
			semconv.ServiceVersion(build.Version),
			semconv.ServiceInstanceID(build.GlobalInstanceId),
			semconv.HostName(hostName),
		),
	)
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTraceExporter(ctx context.Context) (tracesdk.SpanExporter, error) {
	return traceExport.New(
		ctx,
		traceExport.WithRetry(traceExport.RetryConfig{
			Enabled:         true,
			InitialInterval: 5 * time.Second,
			MaxInterval:     30 * time.Second,
			MaxElapsedTime:  time.Minute,
		}),
		traceExport.WithCompression(traceExport.GzipCompression),
	)
}

func newMetricExporter(ctx context.Context) (metric.Exporter, error) {
	return metricExport.New(
		ctx,
		metricExport.WithRetry(metricExport.RetryConfig{
			Enabled:         true,
			InitialInterval: 5 * time.Second,
			MaxInterval:     30 * time.Second,
			MaxElapsedTime:  time.Minute,
		}),
		metricExport.WithCompression(metricExport.GzipCompression),
	)
}

func newLogExporter(ctx context.Context) (log.Exporter, error) {
	return logsExport.New(
		ctx,
		logsExport.WithRetry(logsExport.RetryConfig{
			Enabled:         true,
			InitialInterval: 5 * time.Second,
			MaxInterval:     30 * time.Second,
			MaxElapsedTime:  time.Minute,
		}),
		logsExport.WithCompression(logsExport.GzipCompression),
	)
}

func newTracerProvider(ctx context.Context, res *resource.Resource) (*tracesdk.TracerProvider, error) {
	exp, err := newTraceExporter(ctx)
	if err != nil {
		return nil, err
	}
	return tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(res),
	), nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource) (*metric.MeterProvider, error) {
	exp, err := newMetricExporter(ctx)
	if err != nil {
		return nil, err
	}
	return metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(
			exp,
			metric.WithInterval(30*time.Second),
			metric.WithTimeout(10*time.Second),
		)),
		metric.WithResource(res),
	), nil
}

func newLoggerProvider(ctx context.Context, res *resource.Resource) (*log.LoggerProvider, error) {
	exp, err := newLogExporter(ctx)
	if err != nil {
		return nil, err
	}
	return log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(exp)),
		log.WithResource(res),
	), nil
}

func SetupOTelSDK(ctx context.Context) (err error) {
	var shutdownFuncs []func(context.Context) error

	shutdown := func() error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown())
	}

	otel.SetTextMapPropagator(newPropagator())

	res, err := newResource()
	if err != nil {
		handleErr(err)
		return
	}

	tracerProvider, err := newTracerProvider(ctx, res)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	meterProvider, err := newMeterProvider(ctx, res)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	loggerProvider, err := newLoggerProvider(ctx, res)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)
	logGlobal.SetLoggerProvider(loggerProvider)

	return
}

func StartHostMonitor() error {
	err := host.Start()
	if err != nil {
		return err
	}
	err = runtime.Start()
	if err != nil {
		return err
	}
	return nil
}
