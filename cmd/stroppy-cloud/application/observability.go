package application

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel/attribute"
	logglobal "go.opentelemetry.io/otel/log/global"

	"github.com/gopherex/xlog"
	otelxlog "github.com/gopherex/xlog/contrib/libs/otel"
	"github.com/gopherex/xtrace/contrib/sdk"

	"github.com/stroppy-io/stroppy-cloud/internal/build"
)

/*
OBSERVABILITY: OTel providers first, the logger second — otherwise the log
bridge and instrument scopes bind to nothing.

Without an OTLP endpoint export is off entirely: an SDK raised "into the
void" would only hammer a collector that does not exist.
*/

// setupObservability raises traces/metrics/logs export (when there is
// somewhere to export) and the root logger. Returns the logger and the
// exporters' drain for shutdown (no-op without export).
func setupObservability(ctx context.Context, cfg *Config) (*xlog.Logger, sdk.Shutdown, error) {
	otlpOn := cfg.Trace.Enabled && otlpEndpointConfigured()

	traceShutdown := noopShutdown
	if otlpOn {
		var err error
		traceShutdown, err = sdk.Setup(ctx,
			sdk.WithService(build.ServiceName),
			sdk.WithVersion(build.Version),
			sdk.WithInstanceID(build.InstanceID),
			sdk.WithAttributes(
				attribute.String("service.build.commit", build.Commit),
				attribute.String("service.build.time", build.BuildTime),
			),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("otel sdk: %w", err)
		}
	}

	log, err := newLogger(cfg.Log, otlpOn)
	if err != nil {
		return nil, nil, err
	}
	log = log.AppendName(build.ServiceName).With(
		xlog.String("version", build.Version),
		xlog.String("commit", build.Commit),
		xlog.String("instance_id", build.InstanceID),
	)

	if otlpOn {
		if err := sdk.StartHostRuntime(); err != nil {
			log.Warn("host/runtime metrics unavailable", xlog.ErrorCause(err))
		}
	}
	return log, traceShutdown, nil
}

func noopShutdown(context.Context) error { return nil }

func otlpEndpointConfigured() bool {
	for _, name := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
	} {
		if os.Getenv(name) != "" {
			return true
		}
	}
	return false
}

// newLogger builds the root logger: stdout (json/console) with trace_id and
// span_id of the active span; with OTLP on, the same stream is duplicated
// into the log signal so logs and traces converge in the collector.
func newLogger(cfg LogConfig, otlpLogs bool) (*xlog.Logger, error) {
	level, err := xlog.ParseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("parse log level %q: %w", cfg.Level, err)
	}
	baseOpts := []xlog.Option{xlog.WithLevel(level), xlog.WithWriter(os.Stdout)}

	var base xlog.Core
	if cfg.Format == "console" {
		base = xlog.NewConsole(baseOpts...).Core()
	} else {
		base = xlog.NewJSON(baseOpts...).Core()
	}
	core := base
	if otlpLogs {
		bridge := otelxlog.New(logglobal.GetLoggerProvider().Logger(build.ServiceName))
		core = xlog.NewTeeCore(base, bridge)
	}
	return xlog.NewJSON(
		xlog.WithCore(core),
		xlog.WithLevel(level),
		xlog.WithContextFieldExtractor(otelxlog.TraceFields),
		xlog.WithObserver(otelxlog.SpanObserver(xlog.ErrorLevel)),
	), nil
}
