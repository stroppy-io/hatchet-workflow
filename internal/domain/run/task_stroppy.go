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

	// If the user provided an override, skip generation and send it verbatim.
	if t.stroppy.ConfigOverrideJSON != "" {
		nc.Log().Info("using user-provided stroppy config override")
		return t.client.Send(nc, *target, agent.Command{
			Action: agent.ActionRunStroppy,
			Config: agent.StroppyRunConfig{ConfigJSON: t.stroppy.ConfigOverrideJSON},
		})
	}

	settings := t.stroppySettings
	if settings.OTLPEndpoint == "" && t.monitoringURL != "" {
		settings.SetFromMonitoringURL(t.monitoringURL, t.monitoringToken, t.accountID)
	}

	jsonBytes, err := BuildStroppyConfigJSON(t.stroppy, t.dbKind, dbHost, dbPort, settings, t.runID)
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

// BuildStroppyConfigJSON generates the protojson config sent to the stroppy binary.
// Exported so dry-run can show users what config will be applied.
// dbHost/dbPort may be placeholders in dry-run (resolved at actual execution time).
func BuildStroppyConfigJSON(s types.StroppyConfig, dbKind types.DatabaseKind, dbHost string, dbPort int, settings types.StroppySettings, runID string) ([]byte, error) {
	script := s.Script
	if script == "" {
		script = s.Workload
	}
	if script == "" {
		script = "tpcc/procs"
	}

	var driverURL, driverType string
	switch dbKind {
	case types.DatabasePostgres:
		driverURL = fmt.Sprintf("postgresql://postgres@%s:%d/postgres?sslmode=disable", dbHost, dbPort)
		driverType = "postgres"
	case types.DatabaseMySQL:
		driverURL = fmt.Sprintf("root@tcp(%s:%d)/", dbHost, dbPort)
		driverType = "mysql"
	case types.DatabasePicodata:
		driverURL = fmt.Sprintf("postgres://admin:T0psecret@%s:%d?sslmode=disable", dbHost, dbPort)
		driverType = "picodata"
	case types.DatabaseYDB:
		driverURL = fmt.Sprintf("grpc://%s:%d/Root/testdb", dbHost, dbPort)
		driverType = "ydb"
	default:
		driverURL = fmt.Sprintf("%s:%d", dbHost, dbPort)
		driverType = string(dbKind)
	}

	vus := s.VUs
	if vus == 0 && s.VUSScale > 0 {
		vus = int(s.VUSScale)
	}
	if vus == 0 && s.Workers > 0 {
		vus = s.Workers
	}
	if vus == 0 {
		vus = 1
	}

	poolSize := s.PoolSize
	if poolSize == 0 {
		poolSize = 100
	}
	scaleFactor := s.ScaleFactor
	if scaleFactor == 0 {
		scaleFactor = 1
	}
	duration := s.Duration
	if duration == "" {
		duration = "60s"
	}

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
		Steps:   s.Steps,
		NoSteps: s.NoSteps,
		Global: &stroppypb.GlobalConfig{
			Logger: &stroppypb.LoggerConfig{LogLevel: stroppypb.LoggerConfig_LOG_LEVEL_INFO},
		},
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
		rc.Global.Exporter = &stroppypb.ExporterConfig{OtlpExport: otlpExport}

		svcName := settings.OTLPServiceName
		if svcName == "" {
			svcName = "stroppy"
		}
		rc.Env["OTEL_RESOURCE_ATTRIBUTES"] = fmt.Sprintf("service.name=%s,stroppy.run.id=%s", svcName, runID)
	}

	return protojson.MarshalOptions{
		Multiline:     true,
		Indent:        "  ",
		UseProtoNames: false,
	}.Marshal(rc)
}
