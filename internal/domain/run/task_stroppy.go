package run

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"

	stroppypb "github.com/stroppy-io/stroppy/pkg/common/proto/stroppy"
)

type stroppyInstallTask struct {
	client  agent.Client
	state   *State
	stroppy types.StroppyConfig
}

func (t *stroppyInstallTask) Execute(nc *dag.NodeContext) error {
	target := t.state.StroppyTarget()
	if target == nil {
		return fmt.Errorf("stroppy target not provisioned")
	}
	nc.Log().Info("installing stroppy")
	return t.client.Send(nc, *target, agent.Command{
		Action: agent.ActionInstallStroppy,
		Config: agent.StroppyInstallConfig{Version: t.stroppy.Version},
	})
}

type stroppyRunTask struct {
	client          agent.Client
	state           *State
	stroppy         types.StroppyConfig
	stroppySettings types.StroppySettings
	dbKind          types.DatabaseKind
	runID           string
	monitoringURL   string
	monitoringToken string
	accountID       int32
}

func (t *stroppyRunTask) Execute(nc *dag.NodeContext) error {
	target := t.state.StroppyTarget()
	if target == nil {
		return fmt.Errorf("stroppy target not provisioned")
	}
	dbHost, dbPort := t.state.DBEndpoint()
	nc.Log().Info(fmt.Sprintf("running stroppy test, db_endpoint=%s:%d", dbHost, dbPort))

	// Resolve script: new field takes priority, fall back to deprecated Workload.
	script := t.stroppy.Script
	if script == "" {
		script = t.stroppy.Workload
	}
	if script == "" {
		script = "tpcc/procs"
	}

	// Build driver URL.
	var driverURL, driverType string
	switch t.dbKind {
	case types.DatabasePostgres:
		driverURL = fmt.Sprintf("postgresql://postgres@%s:%d/postgres?sslmode=disable", dbHost, dbPort)
		driverType = "postgres"
	case types.DatabaseMySQL:
		driverURL = fmt.Sprintf("root@tcp(%s:%d)/", dbHost, dbPort)
		driverType = "mysql"
	case types.DatabasePicodata:
		// Picodata stroppy driver connects via PostgreSQL wire protocol (pg port).
		driverURL = fmt.Sprintf("postgres://admin:T0psecret@%s:%d?sslmode=disable", dbHost, dbPort)
		driverType = "picodata"
	case types.DatabaseYDB:
		driverURL = fmt.Sprintf("grpc://%s:%d/Root/testdb", dbHost, dbPort)
		driverType = "ydb"
	default:
		driverURL = fmt.Sprintf("%s:%d", dbHost, dbPort)
		driverType = string(t.dbKind)
	}

	// Resolve VUs: new field, then deprecated VUSScale, then deprecated Workers.
	vus := t.stroppy.VUs
	if vus == 0 && t.stroppy.VUSScale > 0 {
		vus = int(t.stroppy.VUSScale)
	}
	if vus == 0 && t.stroppy.Workers > 0 {
		vus = t.stroppy.Workers
	}
	if vus == 0 {
		vus = 1
	}

	poolSize := t.stroppy.PoolSize
	if poolSize == 0 {
		poolSize = 100
	}
	scaleFactor := t.stroppy.ScaleFactor
	if scaleFactor == 0 {
		scaleFactor = 1
	}
	duration := t.stroppy.Duration
	if duration == "" {
		duration = "60s"
	}

	// Build stroppy RunConfig protobuf.
	maxConns := int32(poolSize)
	rc := &stroppypb.RunConfig{
		Version: "1",
		Script:  &script,
		Drivers: map[uint32]*stroppypb.DriverRunConfig{
			0: {
				DriverType: driverType,
				Url:        driverURL,
				Pool: &stroppypb.DriverRunConfig_PoolConfig{
					MaxConns: &maxConns,
					MinConns: &maxConns,
				},
			},
		},
		Env: map[string]string{
			"SCALE_FACTOR": fmt.Sprintf("%d", scaleFactor),
			"POOL_SIZE":    fmt.Sprintf("%d", poolSize),
		},
		K6Args:  []string{"--vus", fmt.Sprintf("%d", vus), "--duration", duration},
		Steps:   t.stroppy.Steps,
		NoSteps: t.stroppy.NoSteps,
	}

	// Global config — logger + OTLP exporter.
	rc.Global = &stroppypb.GlobalConfig{
		Logger: &stroppypb.LoggerConfig{
			LogLevel: stroppypb.LoggerConfig_LOG_LEVEL_INFO,
		},
	}

	settings := t.stroppySettings
	if settings.OTLPEndpoint == "" && t.monitoringURL != "" {
		settings.SetFromMonitoringURL(t.monitoringURL, t.monitoringToken, t.accountID)
	}
	if settings.OTLPEndpoint != "" {
		insecure := settings.OTLPInsecure
		endpoint := settings.OTLPEndpoint
		urlPath := settings.OTLPURLPath
		metricPrefix := settings.OTLPMetricPrefix
		otlpExport := &stroppypb.OtlpExport{
			OtlpHttpEndpoint:        &endpoint,
			OtlpHttpExporterUrlPath: &urlPath,
			OtlpEndpointInsecure:    &insecure,
			OtlpMetricsPrefix:       &metricPrefix,
		}
		if settings.OTLPHeaders != "" {
			otlpExport.OtlpHeaders = &settings.OTLPHeaders
		}
		rc.Global.Exporter = &stroppypb.ExporterConfig{
			OtlpExport: otlpExport,
		}

		// OTEL resource attributes for run correlation in VictoriaMetrics.
		svcName := settings.OTLPServiceName
		if svcName == "" {
			svcName = "stroppy"
		}
		rc.Env["OTEL_RESOURCE_ATTRIBUTES"] = fmt.Sprintf("service.name=%s,stroppy.run.id=%s", svcName, t.runID)
	}

	// Serialize to JSON via protojson (camelCase field names as stroppy expects).
	jsonBytes, err := protojson.MarshalOptions{
		Multiline:     true,
		Indent:        "  ",
		UseProtoNames: false, // camelCase
	}.Marshal(rc)
	if err != nil {
		return fmt.Errorf("marshal stroppy config: %w", err)
	}

	return t.client.Send(nc, *target, agent.Command{
		Action: agent.ActionRunStroppy,
		Config: agent.StroppyRunConfig{
			ConfigJSON: string(jsonBytes),
		},
	})
}
