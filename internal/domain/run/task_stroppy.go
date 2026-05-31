package run

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"

	stroppypb "github.com/stroppy-io/stroppy/pkg/common/proto/stroppy"
)

// Sentinel tokens rendered in dry-run previews of the stroppy config. The real
// DB endpoint is only known at execution time; the preview embeds these tokens
// so that user-edited overrides can still be substituted before being sent to
// the stroppy binary.
const (
	DBHostPlaceholder = "__STROPPY_DB_HOST__"
	DBPortPlaceholder = "__STROPPY_DB_PORT__"
	DBPathPlaceholder = "__STROPPY_DB_PATH__"
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
	return sendSeq(nc, t.client, *target, stroppyInstallCmd(t.stroppy.Version))
}

// stroppyWorkDir is where the stroppy runner writes its config + workload files.
const stroppyWorkDir = "/tmp/stroppy-run"

// stroppyInstallCmd builds the (idempotent) stroppy download command. Released
// versions are skipped when the pre-installed binary already matches; commit-
// pinned versions always (re)install the exact SHA. Ported from the old agent
// installStroppy — the version-check is now baked into the bash so the agent
// stays dumb.
func stroppyInstallCmd(version string) agent.Command {
	if version == "" {
		version = types.DefaultStroppySettings().Version
	}

	// Commit-pinned: download the raw binary from the per-commit pre-release
	// (tag `nightly-<short_sha>` published by stroppy-io/stroppy CI). Always
	// reinstall to honour the exact SHA.
	if sha, ok := strings.CutPrefix(version, "commit:"); ok {
		short := sha
		if len(short) > 7 {
			short = short[:7]
		}
		dlURL := fmt.Sprintf("https://github.com/stroppy-io/stroppy/releases/download/nightly-%s/stroppy", short)
		script := fmt.Sprintf(
			`curl -fsSL --connect-timeout 20 --max-time 120 --retry 3 --retry-delay 5 --retry-connrefused --retry-max-time 300 %q -o /usr/local/bin/stroppy && chmod +x /usr/local/bin/stroppy`,
			dlURL)
		return runCmd("install_stroppy", script)
	}

	// Released version: accept the pre-installed image binary only if its
	// version matches what the run requested (the agent image ships a pinned
	// stroppy; skipping unconditionally would silently use a stale binary and
	// miss metrics newer releases introduced).
	want := strings.TrimPrefix(version, "v")
	dlURL := fmt.Sprintf("https://github.com/stroppy-io/stroppy/releases/download/v%s/stroppy_linux_amd64.tar.gz", version)
	script := fmt.Sprintf(`if which stroppy >/dev/null 2>&1 && [ "$(stroppy version 2>&1 | head -1 | awk '{print $2}' | sed 's/^v//')" = %q ]; then
  echo "stroppy %s already installed"
  exit 0
fi
curl -fsSL --connect-timeout 20 --max-time 120 --retry 3 --retry-delay 5 --retry-connrefused --retry-max-time 300 %q -o /tmp/stroppy.tar.gz && \
  tar xzf /tmp/stroppy.tar.gz -C /tmp && \
  cp /tmp/stroppy /usr/local/bin/stroppy && \
  chmod +x /usr/local/bin/stroppy && \
  rm -rf /tmp/stroppy*`, want, want, dlURL)
	return runCmd("install_stroppy", script)
}

// stroppyRunCmds builds the primitive sequence that writes the stroppy config +
// workload files into stroppyWorkDir and runs the benchmark from there (so
// uploaded SQL file names resolve through stroppy's normal cwd lookup).
func stroppyRunCmds(configJSON string, files []types.WorkloadFile) ([]agent.Command, error) {
	cmds := []agent.Command{
		runCmd("run_stroppy", fmt.Sprintf("rm -rf %s && mkdir -p %s", stroppyWorkDir, stroppyWorkDir)),
	}
	for _, f := range files {
		name, err := safeWorkloadFileName(f.Name)
		if err != nil {
			return nil, err
		}
		cmds = append(cmds, writeFile("run_stroppy", stroppyWorkDir+"/"+name, f.Content))
	}
	cmds = append(cmds,
		writeFile("run_stroppy", stroppyWorkDir+"/stroppy-config.json", configJSON),
		runCmd("run_stroppy", fmt.Sprintf("cd %s && stroppy run -f stroppy-config.json", stroppyWorkDir)),
	)
	return cmds, nil
}

// safeWorkloadFileName rejects path-traversal / absolute names. Ported verbatim
// from the old agent.
func safeWorkloadFileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("workload file name is required")
	}
	clean := filepath.Clean(name)
	if clean != name || filepath.Base(name) != name || strings.Contains(name, "\x00") {
		return "", fmt.Errorf("invalid workload file name %q", name)
	}
	return name, nil
}

type stroppyRunTask struct {
	client          agent.Client
	state           *State
	stroppy         types.StroppyConfig
	stroppySettings types.StroppySettings
	dbKind          types.DatabaseKind
	dbCfg           types.DatabaseConfig // needed for managed YDB to pull DatabasePath into the URL
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

	settings := t.stroppySettings
	if settings.OTLPEndpoint == "" && t.monitoringURL != "" {
		settings.SetFromMonitoringURL(t.monitoringURL, t.monitoringToken, t.accountID)
	}

	// If the user provided an override: substitute DB endpoint sentinels with the
	// real host/port, then inject tenant OTLP settings if the override doesn't
	// already carry an exporter. This keeps user edits (env, steps, k6_args, etc.)
	// while preserving automatic metrics export.
	if t.stroppy.ConfigOverrideJSON != "" {
		nc.Log().Info("using user-provided stroppy config override")
		cfgJSON := t.stroppy.ConfigOverrideJSON
		cfgJSON = strings.ReplaceAll(cfgJSON, DBHostPlaceholder, dbHost)
		cfgJSON = strings.ReplaceAll(cfgJSON, DBPortPlaceholder, strconv.Itoa(dbPort))

		var rc stroppypb.RunConfig
		if err := protojson.Unmarshal([]byte(cfgJSON), &rc); err != nil {
			return fmt.Errorf("parse stroppy config override: %w", err)
		}
		injectOTLP(&rc, settings, t.runID)
		patched, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(&rc)
		if err != nil {
			return fmt.Errorf("marshal patched stroppy config: %w", err)
		}

		cmds, err := stroppyRunCmds(string(patched), t.stroppy.Files)
		if err != nil {
			return err
		}
		return sendSeq(nc, t.client, *target, cmds...)
	}

	jsonBytes, err := BuildStroppyConfigJSON(t.stroppy, t.dbKind, dbHost, dbPort, settings, t.runID, t.dbCfg)
	if err != nil {
		return fmt.Errorf("marshal stroppy config: %w", err)
	}

	cmds, err := stroppyRunCmds(string(jsonBytes), t.stroppy.Files)
	if err != nil {
		return err
	}
	return sendSeq(nc, t.client, *target, cmds...)
}

// injectOTLP populates the stroppy global exporter + OTEL_RESOURCE_ATTRIBUTES
// env var from tenant OTLP settings. Skipped when settings carry no endpoint
// or when the config already defines an OTLP exporter (user edits win).
func injectOTLP(rc *stroppypb.RunConfig, settings types.StroppySettings, runID string) {
	if settings.OTLPEndpoint == "" {
		return
	}
	if rc.Global == nil {
		rc.Global = &stroppypb.GlobalConfig{
			Logger: &stroppypb.LoggerConfig{LogLevel: stroppypb.LoggerConfig_LOG_LEVEL_INFO},
		}
	}
	if rc.Global.Exporter == nil || rc.Global.Exporter.OtlpExport == nil {
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
	}
	if rc.Env == nil {
		rc.Env = map[string]string{}
	}
	if _, ok := rc.Env["OTEL_RESOURCE_ATTRIBUTES"]; !ok {
		svcName := settings.OTLPServiceName
		if svcName == "" {
			svcName = "stroppy"
		}
		rc.Env["OTEL_RESOURCE_ATTRIBUTES"] = fmt.Sprintf("service.name=%s,stroppy.run.id=%s", svcName, runID)
	}
}

// dbDriverURL formats the connection URL and stroppy driver type for a given
// (kind, protocol). host/port are strings so dry-run can pass sentinel tokens.
//
// All routing flows through the types.Protocols registry: adding a new wire
// format means a one-line entry there, not a new branch here. For protocols
// that need credentials baked into the URL (postgres' "postgres@", picodata's
// "admin:T0psecret@") we splice those in around the metadata's URL prefix.
func dbDriverURL(dbKind types.DatabaseKind, protocol types.Protocol, host, port string, dbCfg types.DatabaseConfig) (string, string) {
	if protocol == "" {
		protocol = types.DefaultProtocol(dbKind)
	}
	meta, ok := types.Protocols[protocol]
	if !ok {
		// Unknown protocol — fall back to a generic host:port and the kind
		// as the driver type. Keeps validate.go responsible for rejecting
		// nonsense before we get here.
		return fmt.Sprintf("%s:%s", host, port), string(dbKind)
	}
	url := meta.FormatURL(host, port)
	// A couple of historical URLs carry credentials in the userinfo slot.
	// Splice them in here so the Protocols registry stays simple.
	switch protocol {
	case types.ProtocolPG:
		url = fmt.Sprintf("postgresql://postgres@%s:%s/postgres?sslmode=disable", host, port)
	case types.ProtocolPicodata:
		url = fmt.Sprintf("postgres://admin:T0psecret@%s:%s?sslmode=disable", host, port)
	case types.ProtocolYDBGRPCS:
		// Managed YDB needs the database path as a query parameter; the
		// path is dynamic (filled by terraform output) and lives in the
		// topology, not the protocol registry.
		dbPath := DBPathPlaceholder
		if dbCfg.YDBManaged != nil && dbCfg.YDBManaged.DatabasePath != "" {
			dbPath = dbCfg.YDBManaged.DatabasePath
		}
		url = fmt.Sprintf("grpcs://%s:%s/?database=%s", host, port, dbPath)
	}
	return url, meta.DriverType
}

// BuildStroppyConfigJSON generates the protojson config sent to the stroppy binary.
// Exported so dry-run can show users what config will be applied.
// When dbHost=="" and dbPort==0, the generated config embeds sentinel tokens
// (DBHostPlaceholder/DBPortPlaceholder) that stroppyRunTask substitutes at run time.
func BuildStroppyConfigJSON(s types.StroppyConfig, dbKind types.DatabaseKind, dbHost string, dbPort int, settings types.StroppySettings, runID string, dbCfg types.DatabaseConfig) ([]byte, error) {
	script := s.Script
	if script == "" {
		script = s.Workload
	}
	if script == "" {
		script = "tpcc/procs"
	}

	// Default the protocol from the kind when the run config didn't pin
	// one — preserves behaviour for runs created before the protocol axis
	// landed.
	protocol := s.Protocol
	if protocol == "" {
		protocol = types.DefaultProtocol(dbKind)
	}

	hostTok, portTok := dbHost, strconv.Itoa(dbPort)
	if dbHost == "" && dbPort == 0 {
		hostTok, portTok = DBHostPlaceholder, DBPortPlaceholder
	}
	driverURL, driverType := dbDriverURL(dbKind, protocol, hostTok, portTok, dbCfg)

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
	iterations := s.Iterations
	if iterations <= 0 {
		iterations = 1
	}
	k6Mode := strings.ToLower(strings.TrimSpace(s.K6Mode))
	if k6Mode == "" {
		k6Mode = "duration"
	}
	quiet := true
	if s.Quiet != nil {
		quiet = *s.Quiet
	}
	k6Args := make([]string, 0, 8)
	if quiet {
		k6Args = append(k6Args, "-q")
	}
	k6Args = append(k6Args, "--vus", fmt.Sprintf("%d", vus))
	if k6Mode == "iterations" {
		k6Args = append(k6Args, "--iterations", fmt.Sprintf("%d", iterations))
	} else {
		k6Args = append(k6Args, "--duration", duration)
	}
	if s.NoThresholds {
		k6Args = append(k6Args, "--no-thresholds")
	}
	defaultInsertMethod := strings.TrimSpace(s.DefaultInsertMethod)
	if defaultInsertMethod == "" {
		defaultInsertMethod = "native"
	}

	var sqlPtr *string
	if s.SQL != "" {
		sql := s.SQL
		sqlPtr = &sql
	}

	maxConns := int32(poolSize)
	rc := &stroppypb.RunConfig{
		Version: "1",
		Script:  &script,
		Sql:     sqlPtr,
		Drivers: map[uint32]*stroppypb.DriverRunConfig{
			0: {
				DriverType:          driverType,
				Url:                 driverURL,
				DefaultInsertMethod: defaultInsertMethod,
				Pool: &stroppypb.DriverRunConfig_PoolConfig{
					MaxConns: &maxConns,
					MinConns: &maxConns,
				},
			},
		},
		Env: func() map[string]string {
			env := map[string]string{
				"SCALE_FACTOR": fmt.Sprintf("%d", scaleFactor),
				"POOL_SIZE":    fmt.Sprintf("%d", poolSize),
			}
			// Tell stroppy how many parallel loaders to use during the seed
			// phase — sized to the runner's vCPU count. With LOAD_WORKERS
			// unset stroppy falls back to a small builtin default that
			// dramatically under-utilises a beefy runner.
			if s.Machine != nil && s.Machine.CPUs > 0 {
				env["LOAD_WORKERS"] = fmt.Sprintf("%d", s.Machine.CPUs)
			}
			for k, v := range s.Env {
				key := strings.ToUpper(strings.TrimSpace(k))
				if key != "" {
					env[key] = v
				}
			}
			return env
		}(),
		K6Args:  k6Args,
		Steps:   s.Steps,
		NoSteps: s.NoSteps,
		Global: &stroppypb.GlobalConfig{
			Logger: &stroppypb.LoggerConfig{LogLevel: stroppypb.LoggerConfig_LOG_LEVEL_INFO},
		},
	}

	injectOTLP(rc, settings, runID)

	return protojson.MarshalOptions{
		Multiline:     true,
		Indent:        "  ",
		UseProtoNames: false,
	}.Marshal(rc)
}
