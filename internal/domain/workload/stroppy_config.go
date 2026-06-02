package workload

import (
	"fmt"
	"net/url"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"

	stroppypb "github.com/stroppy-io/stroppy/pkg/common/proto/stroppy"
)

// Sentinel tokens rendered into the stroppy driver URL. The real DB endpoint is
// only known once the database component is provisioned, so the rendered config
// embeds these and the run command substitutes them at execution time.
const (
	DBHostPlaceholder = "__STROPPY_DB_HOST__"
	DBPortPlaceholder = "__STROPPY_DB_PORT__"
	// DBDatabasePlaceholder is the sentinel for a managed database's database
	// path (e.g. a Managed YDB DatabasePath). It is rendered into the stroppy
	// URL's `?database=` query for managed YDB and substituted at build time
	// from the connected DB endpoint's database_path label. Self-hosted YDB
	// carries no database_path label, so the whole `?database=` query is
	// dropped (matching the main branch: managed → has ?database=,
	// self-hosted → none).
	DBDatabasePlaceholder = "__STROPPY_DB_DATABASE__"

	// dbDatabaseLabel is the endpoint/machine label key the infrastructure
	// state stamps with a managed database's path (see workflows/deployment.go
	// managedYDBMachineState).
	dbDatabaseLabel = "database_path"
)

// monitorAccountID matches deployment/monitor.go: vmagent and vector both ship
// to AccountID 0 through the gateway's /insert/* relay. Stroppy's OTLP metrics
// land in the same VictoriaMetrics tenant.
const monitorAccountID = 0

// buildStroppyRunConfig translates a cloud-side domain.Workload into the stroppy
// binary's RunConfig (the JSON the stroppy CLI consumes) and injects the OTLP
// exporter so the k6/stroppy metrics (`<runID>_vus`, `_iterations`, …) reach
// VictoriaMetrics. serverAddr/runID/bearerToken are read from the topology-spec
// labels (the same channel the monitor collector phase uses).
func buildStroppyRunConfig(input *domain.Workload, serverAddr, runID, bearerToken, databasePath string) *stroppypb.RunConfig {
	script := strings.TrimSpace(input.GetScript())
	if script == "" {
		script = "tpcc/procs"
	}

	var scriptPtr *string
	if script != "" {
		s := script
		scriptPtr = &s
	}
	var sqlPtr *string
	if sql := strings.TrimSpace(input.GetSql()); sql != "" {
		s := sql
		sqlPtr = &s
	}

	driverType, driverURL := driverTypeURL(input.GetProtocol())
	driverURL = resolveDatabasePath(driverURL, databasePath)

	params := input.GetParameters()
	poolSize := int32(params.GetPoolSize())
	if poolSize == 0 {
		poolSize = 100
	}
	scaleFactor := params.GetScaleFactor()
	if scaleFactor == 0 {
		scaleFactor = 1
	}
	insertMethod := strings.TrimSpace(params.GetDefaultInsertMethod())
	if insertMethod == "" {
		insertMethod = "native"
	}

	maxConns := poolSize
	rc := &stroppypb.RunConfig{
		Version: "1",
		Script:  scriptPtr,
		Sql:     sqlPtr,
		Drivers: map[uint32]*stroppypb.DriverRunConfig{
			0: {
				DriverType:          driverType,
				Url:                 driverURL,
				DefaultInsertMethod: insertMethod,
				Pool: &stroppypb.DriverRunConfig_PoolConfig{
					MaxConns: &maxConns,
					MinConns: &maxConns,
				},
			},
		},
		Env:     stroppyEnv(params, scaleFactor, poolSize),
		K6Args:  k6Args(input.GetExecution()),
		Steps:   params.GetSteps(),
		NoSteps: params.GetNoSteps(),
		Global: &stroppypb.GlobalConfig{
			RunId:  runID,
			Logger: &stroppypb.LoggerConfig{LogLevel: stroppypb.LoggerConfig_LOG_LEVEL_INFO},
		},
	}

	injectOTLP(rc, serverAddr, runID, bearerToken)
	return rc
}

// stroppyEnv assembles the k6 script env: SCALE_FACTOR/POOL_SIZE defaults plus
// the user's workload-parameter env overrides (keys uppercased to match
// stroppy's env contract).
func stroppyEnv(params *domain.Workload_Parameters, scaleFactor float64, poolSize int32) map[string]string {
	env := map[string]string{
		"SCALE_FACTOR": trimFloat(scaleFactor),
		"POOL_SIZE":    fmt.Sprintf("%d", poolSize),
	}
	for k, v := range params.GetEnv() {
		key := strings.ToUpper(strings.TrimSpace(k))
		if key != "" {
			env[key] = v
		}
	}
	return env
}

// k6Args mirrors the k6 execution profile (vus + duration|iterations + quiet +
// no-thresholds) into raw "k6 run" args.
func k6Args(exec *domain.Workload_Execution) []string {
	vus := exec.GetVus()
	if vus == 0 {
		vus = 1
	}
	args := make([]string, 0, 8)
	if exec.GetQuiet() {
		args = append(args, "-q")
	}
	args = append(args, "--vus", fmt.Sprintf("%d", vus))
	if iterations := exec.GetIterations(); iterations > 0 {
		args = append(args, "--iterations", fmt.Sprintf("%d", iterations))
	} else {
		duration := strings.TrimSpace(exec.GetDuration())
		if duration == "" {
			duration = "60s"
		}
		args = append(args, "--duration", duration)
	}
	if exec.GetNoThresholds() {
		args = append(args, "--no-thresholds")
	}
	return args
}

// driverTypeURL maps the workload protocol to a stroppy driver type and a
// connection URL templated with the DB host/port sentinels (substituted at run
// time once the DB endpoint is known).
func driverTypeURL(protocol domain.Workload_Protocol) (string, string) {
	host, port := DBHostPlaceholder, DBPortPlaceholder
	switch protocol {
	case domain.Workload_PROTOCOL_PG, domain.Workload_PROTOCOL_COCKROACH:
		return "postgres", fmt.Sprintf("postgresql://postgres@%s:%s/postgres?sslmode=disable", host, port)
	case domain.Workload_PROTOCOL_MYSQL:
		return "mysql", fmt.Sprintf("%s:%s", host, port)
	case domain.Workload_PROTOCOL_PICODATA:
		return "picodata", fmt.Sprintf("postgres://admin:T0psecret@%s:%s?sslmode=disable", host, port)
	case domain.Workload_PROTOCOL_YDB_GRPC:
		return "ydb", fmt.Sprintf("grpc://%s:%s/", host, port)
	case domain.Workload_PROTOCOL_YDB_GRPCS:
		// Managed YDB needs the database path as a `?database=` query (the path
		// is dynamic — terraform output ydb_database_path — and lives on the
		// connected DB endpoint's database_path label). Self-hosted YDB has no
		// such label, so resolveDatabasePath drops the whole query. The
		// placeholder is substituted (or dropped) at build time once the DB
		// endpoint is resolved.
		return "ydb", fmt.Sprintf("grpcs://%s:%s/?database=%s", host, port, DBDatabasePlaceholder)
	default:
		return "postgres", fmt.Sprintf("postgresql://postgres@%s:%s/postgres?sslmode=disable", host, port)
	}
}

// resolveDatabasePath substitutes the managed-database path into the stroppy
// driver URL's `?database=` query. When databasePath is non-empty (managed
// YDB), DBDatabasePlaceholder is replaced verbatim with the path so the URL
// ends with `?database=<path>` (matching main:
// fmt.Sprintf("grpcs://%s:%s/?database=%s", host, port, dbPath)). When it is
// empty (self-hosted YDB, or any DB that never templated the placeholder), the
// whole `?database=<placeholder>` query is dropped, leaving the URL with no
// `?database=` — matching the main branch (managed → has ?database=,
// self-hosted → none). URLs that never carried the placeholder are returned
// unchanged.
func resolveDatabasePath(driverURL, databasePath string) string {
	if !strings.Contains(driverURL, DBDatabasePlaceholder) {
		return driverURL
	}
	if databasePath == "" {
		// Drop the trailing `?database=<placeholder>` query entirely.
		return strings.Replace(driverURL, "?database="+DBDatabasePlaceholder, "", 1)
	}
	return strings.Replace(driverURL, DBDatabasePlaceholder, databasePath, 1)
}

// injectOTLP populates the stroppy global exporter + OTEL_RESOURCE_ATTRIBUTES
// env var so the runner ships k6/stroppy metrics. It is a no-op when no server
// address is known (local/docker runs) or when the config already carries an
// OTLP exporter (user edits win). The endpoint is derived from the agent's
// server address (gateway), the same path the vmagent /insert relay uses.
func injectOTLP(rc *stroppypb.RunConfig, serverAddr, runID, bearerToken string) {
	serverAddr = strings.TrimRight(strings.TrimSpace(serverAddr), "/")
	if serverAddr == "" {
		return
	}
	if rc.Global == nil {
		rc.Global = &stroppypb.GlobalConfig{
			Logger: &stroppypb.LoggerConfig{LogLevel: stroppypb.LoggerConfig_LOG_LEVEL_INFO},
		}
	}
	if rc.Global.Exporter == nil || rc.Global.Exporter.OtlpExport == nil {
		endpoint, insecure := otlpEndpoint(serverAddr)
		urlPath := fmt.Sprintf("/insert/%d/opentelemetry/v1/metrics", monitorAccountID)
		// Metric prefix = runID (dashes→underscores) + "_". Stroppy prepends it
		// to every metric name, so `vus` becomes `<runID_>_vus`, which is what
		// metrics/queries.go RenderQuery expects via its %p substitution
		// (%p = runID with dashes→underscores; queries read `%p_vus`).
		metricPrefix := strings.ReplaceAll(runID, "-", "_") + "_"

		otlpExport := &stroppypb.OtlpExport{
			OtlpHttpEndpoint:        &endpoint,
			OtlpHttpExporterUrlPath: &urlPath,
			OtlpEndpointInsecure:    &insecure,
			OtlpMetricsPrefix:       &metricPrefix,
		}
		if bearerToken != "" {
			headers := "Authorization=Bearer " + bearerToken
			otlpExport.OtlpHeaders = &headers
		}
		rc.Global.Exporter = &stroppypb.ExporterConfig{OtlpExport: otlpExport}
	}

	if rc.Env == nil {
		rc.Env = map[string]string{}
	}
	if _, ok := rc.Env["OTEL_RESOURCE_ATTRIBUTES"]; !ok {
		rc.Env["OTEL_RESOURCE_ATTRIBUTES"] = fmt.Sprintf("service.name=stroppy,stroppy.run.id=%s", runID)
	}
}

// otlpEndpoint splits the server address into the host:port stroppy's OTLP HTTP
// exporter expects (no scheme) and reports whether transport security is off
// (http / no scheme = insecure).
func otlpEndpoint(serverAddr string) (string, bool) {
	if u, err := url.Parse(serverAddr); err == nil && u.Host != "" {
		return u.Host, u.Scheme != "https"
	}
	// No scheme: serverAddr is already host:port. Treat as insecure (http).
	return serverAddr, true
}

func trimFloat(f float64) string {
	s := fmt.Sprintf("%g", f)
	return s
}

// renderStroppyConfigJSON marshals the stroppy RunConfig built from the workload
// (with OTLP injected from the topology labels) to the protojson the stroppy
// binary loads.
func renderStroppyConfigJSON(input *domain.Workload, labels map[string]string, databasePath string) string {
	serverAddr := strings.TrimRight(labels[deploymentbuilder.LabelServerAddr], "/")
	runID := labels[deploymentbuilder.LabelRunID]
	bearerToken := labels[deploymentbuilder.LabelMonitorBearerToken]

	rc := buildStroppyRunConfig(input, serverAddr, runID, bearerToken, databasePath)
	data, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(rc)
	if err != nil {
		return "{}\n"
	}
	return string(data) + "\n"
}
