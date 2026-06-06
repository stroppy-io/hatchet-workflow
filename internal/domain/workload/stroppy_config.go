package workload

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbcredentials"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/metrics"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"

	stroppypb "github.com/stroppy-io/stroppy/pkg/common/proto/stroppy"
)

// Sentinel tokens rendered into dry-run previews and user overrides. The real
// deployment plan must carry concrete DB runtime addresses by the time the
// workload runner writes stroppy-config.json.
const (
	DBHostPlaceholder     = "__STROPPY_DB_HOST__"
	DBPortPlaceholder     = "__STROPPY_DB_PORT__"
	DBUserPlaceholder     = "__STROPPY_DB_USER__"
	DBPasswordPlaceholder = "__STROPPY_DB_PASSWORD__"
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

type databaseTarget struct {
	Host         string
	Port         uint32
	User         string
	Password     string
	DatabasePath string
}

type protocolMeta struct {
	driverType string
	port       uint32
	urlScheme  string
	urlTail    string
}

func (p protocolMeta) formatURL(host, port string) string {
	if p.driverType == "mysql" {
		return fmt.Sprintf("root@tcp(%s:%s)%s", host, port, p.urlTail)
	}
	return fmt.Sprintf("%s://%s:%s%s", p.urlScheme, host, port, p.urlTail)
}

var workloadProtocols = map[domain.Workload_Protocol]protocolMeta{
	domain.Workload_PROTOCOL_PG:        {driverType: "postgres", port: 5432, urlScheme: "postgresql", urlTail: "/postgres?sslmode=disable"},
	domain.Workload_PROTOCOL_MYSQL:     {driverType: "mysql", port: 3306, urlTail: "/stroppy"},
	domain.Workload_PROTOCOL_PICODATA:  {driverType: "picodata", port: 5432, urlScheme: "postgres", urlTail: "?sslmode=disable"},
	domain.Workload_PROTOCOL_YDB_GRPC:  {driverType: "ydb", port: 2136, urlScheme: "grpc", urlTail: "/Root/testdb"},
	domain.Workload_PROTOCOL_YDB_GRPCS: {driverType: "ydb", port: 2135, urlScheme: "grpcs"},
	domain.Workload_PROTOCOL_COCKROACH: {driverType: "postgres", port: 26257, urlScheme: "postgresql", urlTail: "/defaultdb?sslmode=disable"},
}

var databaseDefaultProtocols = map[domain.Database_Kind]domain.Workload_Protocol{
	domain.Database_KIND_MYSQL:       domain.Workload_PROTOCOL_MYSQL,
	domain.Database_KIND_MARIADB:     domain.Workload_PROTOCOL_MYSQL,
	domain.Database_KIND_YDB:         domain.Workload_PROTOCOL_YDB_GRPC,
	domain.Database_KIND_YDB_MANAGED: domain.Workload_PROTOCOL_YDB_GRPCS,
	domain.Database_KIND_COCKROACH:   domain.Workload_PROTOCOL_COCKROACH,
	domain.Database_KIND_PICODATA:    domain.Workload_PROTOCOL_PICODATA,
}

func (t databaseTarget) hostToken() string {
	host := strings.TrimSpace(t.Host)
	if host == "" {
		return DBHostPlaceholder
	}
	return host
}

func (t databaseTarget) portToken() string {
	if t.Port == 0 {
		return DBPortPlaceholder
	}
	return strconv.FormatUint(uint64(t.Port), 10)
}

func (t databaseTarget) userToken() string {
	user := strings.TrimSpace(t.User)
	if user == "" {
		return dbcredentials.PostgresUser
	}
	return user
}

func (t databaseTarget) passwordToken() string {
	if t.Password == "" {
		return ""
	}
	return t.Password
}

func (t databaseTarget) replacePlaceholders(text string) string {
	text = strings.ReplaceAll(text, DBHostPlaceholder, t.hostToken())
	text = strings.ReplaceAll(text, DBPortPlaceholder, t.portToken())
	text = strings.ReplaceAll(text, DBUserPlaceholder, t.userToken())
	text = strings.ReplaceAll(text, DBPasswordPlaceholder, t.passwordToken())
	if t.DatabasePath == "" {
		text = strings.ReplaceAll(text, "?database="+DBDatabasePlaceholder, "")
		return strings.ReplaceAll(text, DBDatabasePlaceholder, "")
	}
	return strings.ReplaceAll(text, DBDatabasePlaceholder, t.DatabasePath)
}

func (t databaseTarget) withDefaults(database *domain.Database) databaseTarget {
	if database.GetKind() != domain.Database_KIND_POSTGRES {
		return t
	}
	if strings.TrimSpace(t.User) == "" {
		t.User = dbcredentials.PostgresUser
	}
	if t.Password == "" {
		t.Password = dbcredentials.PostgresPassword
	}
	return t
}

// monitorAccountID matches deployment/monitor.go: vmagent and vector both ship
// to AccountID 0 through the gateway's /insert/* relay. Stroppy's OTLP metrics
// land in the same VictoriaMetrics tenant.
const monitorAccountID = 0

// buildStroppyRunConfig translates a cloud-side domain.Workload into the stroppy
// binary's RunConfig (the JSON the stroppy CLI consumes) and injects the OTLP
// exporter so the k6/stroppy metrics (`<runID>_vus`, `_iterations`, …) reach
// VictoriaMetrics. serverAddr/runID/bearerToken are read from the topology-spec
// labels plus the per-node agent token from deployment RenderContext.
func buildStroppyRunConfig(input *domain.Workload, database *domain.Database, serverAddr, runID, bearerToken string, target databaseTarget, loadWorkers uint32) *stroppypb.RunConfig {
	target = target.withDefaults(database)
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

	driverType, driverURL := driverTypeURL(effectiveProtocol(input.GetProtocol(), database), target)
	driverURL = resolveDatabasePath(driverURL, target.DatabasePath)

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
		Env:     stroppyEnv(params, scaleFactor, poolSize, loadWorkers),
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
func stroppyEnv(params *domain.Workload_Parameters, scaleFactor float64, poolSize int32, loadWorkers uint32) map[string]string {
	env := map[string]string{
		"SCALE_FACTOR": trimFloat(scaleFactor),
		"POOL_SIZE":    fmt.Sprintf("%d", poolSize),
	}
	if loadWorkers > 0 {
		env["LOAD_WORKERS"] = strconv.FormatUint(uint64(loadWorkers), 10)
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
	// Workload.Execution.quiet is a proto3 scalar today, so the API cannot
	// distinguish "unset" from "explicit false". Match the main generator's
	// production default and keep k6 quiet unless the protocol grows presence.
	args := []string{"-q"}
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
// connection URL. Render previews use sentinels; runtime deployment plans use
// concrete DB endpoint host/port from deployment.RuntimeView.
func driverTypeURL(protocol domain.Workload_Protocol, target databaseTarget) (string, string) {
	host, port := target.hostToken(), target.portToken()
	meta, ok := workloadProtocols[protocol]
	if !ok {
		meta = workloadProtocols[domain.Workload_PROTOCOL_PG]
	}

	switch protocol {
	case domain.Workload_PROTOCOL_PG:
		return meta.driverType, fmt.Sprintf("postgresql://%s@%s:%s/postgres?sslmode=disable", postgresUserInfo(target), host, port)
	case domain.Workload_PROTOCOL_PICODATA:
		return meta.driverType, fmt.Sprintf("postgres://admin:T0psecret@%s:%s?sslmode=disable", host, port)
	case domain.Workload_PROTOCOL_YDB_GRPCS:
		// Managed YDB needs the database path as a `?database=` query (the path
		// is dynamic — terraform output ydb_database_path — and lives on the
		// connected DB endpoint's database_path label). Self-hosted YDB has no
		// such label, so resolveDatabasePath drops the whole query. The
		// placeholder is substituted (or dropped) at build time once the DB
		// endpoint is resolved.
		return meta.driverType, fmt.Sprintf("grpcs://%s:%s/?database=%s", host, port, DBDatabasePlaceholder)
	default:
		return meta.driverType, meta.formatURL(host, port)
	}
}

func postgresUserInfo(target databaseTarget) string {
	user := target.userToken()
	if target.Password == "" {
		return url.User(user).String()
	}
	return url.UserPassword(user, target.passwordToken()).String()
}

func effectiveProtocol(protocol domain.Workload_Protocol, database *domain.Database) domain.Workload_Protocol {
	if protocol != domain.Workload_PROTOCOL_UNSPECIFIED {
		return protocol
	}
	if resolved, ok := databaseDefaultProtocols[database.GetKind()]; ok {
		return resolved
	}
	return domain.Workload_PROTOCOL_PG
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
	if runID != "" && rc.Global.RunId == "" {
		rc.Global.RunId = runID
	}
	if rc.Global.Exporter == nil || rc.Global.Exporter.OtlpExport == nil {
		endpoint, insecure := otlpEndpoint(serverAddr)
		urlPath := fmt.Sprintf("/insert/%d/opentelemetry/v1/metrics", monitorAccountID)
		// Stroppy prepends this to every metric name, so `vus` becomes
		// `stroppy_<runID>_vus`. OTEL instrument names must start with a letter.
		metricPrefix := metrics.StroppyMetricPrefix(runID) + "_"

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

func patchStroppyConfigFile(file *common.File, labels map[string]string, target databaseTarget, bearerToken string) (*common.File, error) {
	if file == nil {
		return nil, fmt.Errorf("stroppy config override file is required")
	}
	text, ok := file.GetContent().(*common.File_Text)
	if !ok {
		return nil, fmt.Errorf("stroppy config override must be an inline text file")
	}

	var rc stroppypb.RunConfig
	if err := protojson.Unmarshal([]byte(target.replacePlaceholders(text.Text)), &rc); err != nil {
		return nil, fmt.Errorf("parse stroppy config override: %w", err)
	}
	serverAddr := strings.TrimRight(labels[deploymentbuilder.LabelServerAddr], "/")
	runID := labels[deploymentbuilder.LabelRunID]
	injectOTLP(&rc, serverAddr, runID, bearerToken)
	data, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(&rc)
	if err != nil {
		return nil, fmt.Errorf("marshal stroppy config override: %w", err)
	}

	out := proto.Clone(file).(*common.File)
	out.Content = &common.File_Text{Text: string(data) + "\n"}
	return out, nil
}

// renderStroppyConfigJSON marshals the stroppy RunConfig built from the workload
// (with OTLP injected from the topology labels) to the protojson the stroppy
// binary loads.
func renderStroppyConfigJSON(input *domain.Workload, database *domain.Database, labels map[string]string, target databaseTarget, loadWorkers uint32, bearerToken string) string {
	serverAddr := strings.TrimRight(labels[deploymentbuilder.LabelServerAddr], "/")
	runID := labels[deploymentbuilder.LabelRunID]

	rc := buildStroppyRunConfig(input, database, serverAddr, runID, bearerToken, target, loadWorkers)
	data, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(rc)
	if err != nil {
		return "{}\n"
	}
	return string(data) + "\n"
}
