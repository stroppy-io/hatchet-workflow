package deployment

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

// Monitor phase: agent-side metrics + logs collectors.
//
// When the agent became a set of dumb primitives (CreateDir / WriteFile /
// CallCmd) the executor that used to install node_exporter, the DB exporter,
// vmagent and vector was removed, leaving deployed machines emitting nothing.
// This file rebuilds that collector phase as ordered AgentSteps appended after
// a component's own deploy steps, faithful to the previous executor behaviour:
//
//   - node_exporter on EVERY component machine.
//   - postgres_exporter / mysqld_exporter on database machines (by engine).
//   - vmagent on every machine, scraping the local exporters present on THIS
//     machine, remote-writing to <serverAddr>/insert/0/prometheus/api/v1/write.
//   - vector on every machine, shipping journald + DB logs to
//     <serverAddr>/insert/jsonline with AccountID 0.
//
// Agents reach the monitoring backends ONLY through the server/gateway address
// (the gateway relays /insert/* to vmauth). AccountID is 0 for both metrics and
// logs; per-run isolation is by the stroppy_run_id label / run_id field.
//
// The server address / run id travel from the run config into the TopologySpec
// labels (see internal/domain/run/workflow_config.go). The bearer token is the
// per-node agent token passed through RenderContext, never topology labels.

const (
	// LabelServerAddr is the topology-spec label carrying the control-plane
	// base address agents use to reach binaries and monitoring backends.
	LabelServerAddr = "stroppy.io/server-addr"
	// LabelRunID is the topology-spec label carrying the run id, stamped as an
	// external metrics label (stroppy_run_id) and a log field (run_id).
	LabelRunID = "stroppy.io/run-id"
	// monitorAccountID is the VictoriaMetrics / VictoriaLogs account both the
	// write path (here) and the read path use. Isolation is by run id label.
	monitorAccountID = 0
)

// Binary versions — match internal/domain/types.DefaultMonitoring() on main.
const (
	nodeExporterVersion     = "1.9.1"
	postgresExporterVersion = "0.18.1"
	mysqldExporterVersion   = "0.19.0"
	vectorVersion           = "0.43.1"
	vmagentVersion          = "1.139.0"
)

// curlOpts mirrors the resilient curl settings used by the old executor — the
// github SSL handshakes from inside Yandex Cloud were flaky, so retries matter
// even when downloading through the server binary cache.
const curlOpts = `--connect-timeout 20 --max-time 300 --retry 3 --retry-delay 5 --retry-connrefused --retry-max-time 600`

// monitorParams is the per-machine monitoring context resolved from the
// component, its node, and the topology-spec labels.
type monitorParams struct {
	machineID   string
	runID       string
	serverAddr  string
	bearerToken string
	role        string
	combined    bool
	dbKind      string // "postgres" | "mysql" | "picodata" | "ydb" | "" (none)
}

// monitorParamsFor resolves the monitoring context for one component machine.
// It returns ok=false when the server address is unknown (no place to ship to),
// in which case the caller skips the monitor phase entirely.
func monitorParamsFor(ctx RenderContext) (monitorParams, bool) {
	labels := ctx.Topology.Spec().GetLabels()
	serverAddr := strings.TrimRight(labels[LabelServerAddr], "/")
	if serverAddr == "" {
		return monitorParams{}, false
	}

	engine := ctx.Component.GetEngine()
	dbKind := ""
	switch engine {
	case "postgres", "mysql", "picodata", "ydb", "cockroach":
		// Only emit a DB exporter for the engine roles that actually run a DB
		// server on the machine. Proxy / etcd / coordinator roles share the
		// engine name but host no DB to scrape.
		if isDatabaseRole(engine, ctx.Component.GetRole()) {
			dbKind = engine
		}
	}

	return monitorParams{
		machineID:   ctx.Node.GetId(),
		runID:       labels[LabelRunID],
		serverAddr:  serverAddr,
		bearerToken: ctx.AgentToken,
		role:        ctx.Component.GetRole(),
		combined:    labels["combined"] == "true",
		dbKind:      dbKind,
	}, true
}

// isDatabaseRole reports whether a component runs a scrapeable DB server.
func isDatabaseRole(engine, role string) bool {
	switch engine {
	case "postgres":
		return role == "master" || role == "replica"
	case "mysql":
		return role == "primary" || role == "replica"
	case "picodata":
		return role == "instance"
	case "ydb":
		return role == "storage" || role == "database"
	case "cockroach":
		return role == "node"
	default:
		return false
	}
}

// MonitorSteps builds the ordered collector step list for one component machine.
// Steps start at order 300 so they run after the component's own deploy +
// healthcheck steps (which end at 230). All scripts are idempotent: a machine
// hosting several components re-runs install (guarded by `which`) and restarts
// the daemons against the freshly written config.
func MonitorSteps(ctx RenderContext) []*deploymentpb.AgentStep {
	params, ok := monitorParamsFor(ctx)
	if !ok {
		return nil
	}

	steps := make([]*deploymentpb.AgentStep, 0, 6)
	steps = append(steps,
		CallCmdStep("300_install_collectors", 300, installCollectorsScript(params)),
		CallCmdStep("310_start_exporters", 310, startExportersScript(params)),
		WriteFileStep("320_write_vmagent_scrape", 320, vmagentScrapeFile(params)),
		CallCmdStep("330_start_vmagent", 330, startVmagentScript(params)),
		WriteFileStep("340_write_vector_config", 340, vectorConfigFile(params)),
		CallCmdStep("350_start_vector", 350, startVectorScript()),
	)
	return steps
}

// binURL composes the server binary-cache URL the gateway serves. All collector
// binaries are pulled through it (never github directly) so one cold download
// warms the cache for the whole fleet and avoids flaky direct SSL handshakes.
func binURL(serverAddr, name, version, file string) string {
	return fmt.Sprintf("%s/api/binaries/%s/%s/%s", serverAddr, name, version, file)
}

// installCollectorsScript downloads + installs node_exporter (all machines),
// the DB exporter (database machines), vmagent (all) and vector (all) from the
// server binary cache. Each install is guarded by `which` so re-runs on a
// shared machine are no-ops.
func installCollectorsScript(p monitorParams) string {
	var b strings.Builder
	b.WriteString("set -e\n")

	// node_exporter — every machine.
	neFile := fmt.Sprintf("node_exporter-%s.linux-amd64.tar.gz", nodeExporterVersion)
	fmt.Fprintf(&b, `if ! command -v node_exporter >/dev/null 2>&1; then
  curl -fsSL %s "%s" -o /tmp/node_exporter.tar.gz
  tar xzf /tmp/node_exporter.tar.gz -C /tmp
  cp /tmp/node_exporter-%s.linux-amd64/node_exporter /usr/local/bin/node_exporter
  chmod +x /usr/local/bin/node_exporter
  rm -rf /tmp/node_exporter*
fi
`, curlOpts, binURL(p.serverAddr, "node_exporter", nodeExporterVersion, neFile), nodeExporterVersion)

	// postgres_exporter — postgres database machines.
	if p.dbKind == "postgres" {
		peFile := fmt.Sprintf("postgres_exporter-%s.linux-amd64.tar.gz", postgresExporterVersion)
		fmt.Fprintf(&b, `if ! command -v postgres_exporter >/dev/null 2>&1; then
  if curl -fsSL %s "%s" -o /tmp/postgres_exporter.tar.gz && \
    tar xzf /tmp/postgres_exporter.tar.gz -C /tmp && \
    cp /tmp/postgres_exporter-%s.linux-amd64/postgres_exporter /usr/local/bin/postgres_exporter; then
    chmod +x /usr/local/bin/postgres_exporter
  else
    echo "install postgres_exporter from binary cache failed, falling back to apt"
    apt-get update
    apt-get install -y --no-install-recommends prometheus-postgres-exporter
    systemctl disable --now prometheus-postgres-exporter 2>/dev/null || true
    install -m 0755 /usr/bin/prometheus-postgres-exporter /usr/local/bin/postgres_exporter
  fi
  rm -rf /tmp/postgres_exporter*
fi
`, curlOpts, binURL(p.serverAddr, "postgres_exporter", postgresExporterVersion, peFile), postgresExporterVersion)
	}

	// mysqld_exporter — mysql/mariadb database machines.
	if p.dbKind == "mysql" {
		meFile := fmt.Sprintf("mysqld_exporter-%s.linux-amd64.tar.gz", mysqldExporterVersion)
		fmt.Fprintf(&b, `if ! command -v mysqld_exporter >/dev/null 2>&1; then
  curl -fsSL %s "%s" -o /tmp/mysqld_exporter.tar.gz
  tar xzf /tmp/mysqld_exporter.tar.gz -C /tmp
  cp /tmp/mysqld_exporter-%s.linux-amd64/mysqld_exporter /usr/local/bin/mysqld_exporter
  chmod +x /usr/local/bin/mysqld_exporter
  rm -rf /tmp/mysqld_exporter*
fi
`, curlOpts, binURL(p.serverAddr, "mysqld_exporter", mysqldExporterVersion, meFile), mysqldExporterVersion)
	}

	// vmagent — every machine.
	vaFile := fmt.Sprintf("vmutils-linux-amd64-v%s.tar.gz", vmagentVersion)
	fmt.Fprintf(&b, `if ! command -v vmagent >/dev/null 2>&1; then
  curl -fsSL %s "%s" -o /tmp/vmutils.tar.gz
  tar xzf /tmp/vmutils.tar.gz -C /tmp
  cp /tmp/vmagent-prod /usr/local/bin/vmagent
  chmod +x /usr/local/bin/vmagent
  rm -rf /tmp/vmutils* /tmp/vmagent* /tmp/vmalert* /tmp/vmauth* /tmp/vmbackup* /tmp/vmrestore*
fi
`, curlOpts, binURL(p.serverAddr, "vmagent", vmagentVersion, vaFile))

	// vector — every machine. Best-effort: log shipping is non-critical, so a
	// failed download must not fail the deploy step.
	vecFile := fmt.Sprintf("vector-%s-x86_64-unknown-linux-musl.tar.gz", vectorVersion)
	fmt.Fprintf(&b, `if ! command -v vector >/dev/null 2>&1; then
  if curl -fsSL %s "%s" -o /tmp/vector.tar.gz; then
    tar xzf /tmp/vector.tar.gz -C /tmp
    cp /tmp/vector-x86_64-unknown-linux-musl/bin/vector /usr/local/bin/vector
    chmod +x /usr/local/bin/vector
    rm -rf /tmp/vector*
  else
    echo "install vector failed (logs to VictoriaLogs disabled)"
  fi
fi
`, curlOpts, binURL(p.serverAddr, "vector", vectorVersion, vecFile))

	return b.String()
}

// startExportersScript starts node_exporter and, on database machines, the DB
// exporter. Daemons run via systemd transient units (systemd-run) to match the
// other daemons in this branch (all engines start through systemd). Re-running
// resets the unit so a shared machine converges.
func startExportersScript(p monitorParams) string {
	var b strings.Builder
	b.WriteString("set -e\n")
	b.WriteString(systemdRunUnit("stroppy-node-exporter", "", "/usr/local/bin/node_exporter"))

	switch p.dbKind {
	case "postgres":
		// postgres_exporter connects to the LOCAL postgres over the standard port.
		b.WriteString(systemdRunUnit(
			"stroppy-postgres-exporter",
			"DATA_SOURCE_NAME=postgresql://postgres@localhost:5432/postgres?sslmode=disable",
			"/usr/local/bin/postgres_exporter",
		))
	case "mysql":
		// Create the least-privilege exporter user, then start mysqld_exporter
		// against the local server. Best-effort grant (idempotent). The user is
		// '%' (not 'localhost') because the exporter connects over TCP, so the
		// server sees 127.0.0.1 and a 'localhost' account would never match.
		// MariaDB 11+ ships only the `mariadb` client, so fall back to it when
		// `mysql` is absent, and connect through the local socket as root.
		b.WriteString(`mysqlcli="$(command -v mysql || command -v mariadb || echo mysql)"
"$mysqlcli" -u root --socket=/run/mysqld/mysqld.sock -e "SET sql_log_bin=0; CREATE USER IF NOT EXISTS 'exporter'@'%' IDENTIFIED BY 'exporter' WITH MAX_USER_CONNECTIONS 3; GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'exporter'@'%'; FLUSH PRIVILEGES; SET sql_log_bin=1;" 2>/dev/null || true
`)
		b.WriteString(systemdRunUnit(
			"stroppy-mysqld-exporter",
			"MYSQLD_EXPORTER_PASSWORD=exporter",
			"/usr/local/bin/mysqld_exporter --mysqld.address=127.0.0.1:3306 --mysqld.username=exporter",
		))
	}

	return b.String()
}

// systemdRunUnit emits an idempotent systemd-run invocation for a long-running
// collector. The unit is stopped first so re-runs (shared machine, retries)
// restart cleanly against the current binary/config.
func systemdRunUnit(unit, env, exec string) string {
	setenv := ""
	if env != "" {
		setenv = "--setenv=" + ShellQuote(env) + " "
	}
	return fmt.Sprintf(`systemctl stop %s 2>/dev/null || true
systemd-run --unit=%s --collect %s%s
`, ShellQuote(unit), ShellQuote(unit), setenv, exec)
}

// vmagentScrapeFile renders /etc/vmagent/scrape.yml with the external labels and
// the scrape jobs for the exporters present on THIS machine. Picodata and YDB
// scrape the DB process directly (no separate exporter binary).
func vmagentScrapeFile(p monitorParams) *common.File {
	var b strings.Builder
	b.WriteString("# Generated by stroppy-cloud monitor phase\n")
	b.WriteString("global:\n  scrape_interval: 5s\n")
	fmt.Fprintf(&b, "  external_labels:\n    stroppy_machine_id: '%s'\n", p.machineID)
	if p.runID != "" {
		fmt.Fprintf(&b, "    stroppy_run_id: '%s'\n", p.runID)
	}

	b.WriteString("\nscrape_configs:\n")
	// node_exporter on every machine.
	b.WriteString("  - job_name: node\n    static_configs:\n      - targets: ['localhost:9100']\n")

	switch p.dbKind {
	case "postgres":
		b.WriteString("  - job_name: postgres\n    static_configs:\n      - targets: ['localhost:9187']\n")
	case "mysql":
		b.WriteString("  - job_name: mysql\n    static_configs:\n      - targets: ['localhost:9104']\n")
	case "picodata":
		b.WriteString("  - job_name: picodata\n    metrics_path: /metrics\n    static_configs:\n      - targets: ['localhost:8081']\n")
	case "cockroach":
		// CockroachDB exposes Prometheus metrics natively on the HTTP port at
		// /_status/vars; there is no separate exporter to install.
		b.WriteString("  - job_name: cockroach\n    metrics_path: /_status/vars\n    static_configs:\n      - targets: ['localhost:8080']\n")
	case "ydb":
		writeYDBScrapeJobs(&b, p.role, p.combined)
	}

	return &common.File{
		Info: &common.File_Info{
			Path:          "/etc/vmagent/scrape.yml",
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: b.String()},
	}
}

type ydbCounterGroup struct {
	name string
	path string
	role string
}

type ydbScrapeRole struct {
	name      string
	port      string
	container string
}

func writeYDBScrapeJobs(b *strings.Builder, componentRole string, combined bool) {
	counters := []ydbCounterGroup{
		{name: "ydb", path: "/counters/counters=ydb/name_label=name/prometheus"},
		{name: "auth"},
		{name: "coordinator"},
		{name: "dsproxy"},
		{name: "dsproxy_queue"},
		{name: "dsproxy_percentile"},
		{name: "dsproxynode"},
		{name: "grpc"},
		{name: "interconnect"},
		{name: "kqp", role: "dynamic"},
		{name: "pdisks", role: "static"},
		{name: "processing"},
		{name: "proxy"},
		{name: "storage_pool_stat"},
		{name: "tablets"},
		{name: "utils"},
		{name: "vdisks", role: "static"},
	}
	staticRole := ydbScrapeRole{name: "static", port: "8765", container: "ydb-static"}
	dynamicRole := ydbScrapeRole{name: "dynamic", port: "8766", container: "ydb-dynamic"}
	roles := []ydbScrapeRole{staticRole, dynamicRole}
	switch {
	case componentRole == "storage" && !combined:
		roles = []ydbScrapeRole{staticRole}
	case componentRole == "database":
		roles = []ydbScrapeRole{dynamicRole}
	}
	for _, role := range roles {
		for _, counter := range counters {
			if counter.role != "" && counter.role != role.name {
				continue
			}
			path := counter.path
			if path == "" {
				path = fmt.Sprintf("/counters/counters=%s/prometheus", counter.name)
			}
			fmt.Fprintf(b,
				"  - job_name: ydb_%s_%s\n"+
					"    metrics_path: %s\n"+
					"    static_configs:\n"+
					"      - targets: ['localhost:%s']\n"+
					"        labels:\n"+
					"          container: %s\n"+
					"          counter: %s\n"+
					"    metric_relabel_configs:\n"+
					"      - source_labels: [__name__]\n"+
					"        regex: (.*)\n"+
					"        target_label: __name__\n"+
					"        replacement: %s_$1\n",
				counter.name, role.name, path, role.port, role.container, counter.name, counter.name,
			)
		}
	}
}

// startVmagentScript starts vmagent remote-writing to the gateway's /insert
// relay (AccountID 0). The gateway forwards /insert/* to vmauth/VictoriaMetrics.
func startVmagentScript(p monitorParams) string {
	remoteWrite := fmt.Sprintf("%s/insert/%d/prometheus/api/v1/write", p.serverAddr, monitorAccountID)
	exec := "/usr/local/bin/vmagent " +
		"-promscrape.config=/etc/vmagent/scrape.yml " +
		"-remoteWrite.url=" + ShellQuote(remoteWrite) + " " +
		"-remoteWrite.tmpDataPath=/var/lib/vmagent"
	if p.bearerToken != "" {
		exec += " -remoteWrite.bearerToken=" + ShellQuote(p.bearerToken)
	}
	var b strings.Builder
	b.WriteString("set -e\n")
	b.WriteString("mkdir -p /var/lib/vmagent\n")
	b.WriteString(systemdRunUnit("stroppy-vmagent", "", exec))
	return b.String()
}

// vectorConfigFile renders /etc/vector/vector.yaml: journald + DB-file sources,
// a multiline_join reduce that re-assembles multi-line DB records, an enrich
// remap that tags each event with run_id / machine_id / role / unit, and an HTTP
// sink to the gateway's /insert/jsonline (AccountID 0).
func vectorConfigFile(p monitorParams) *common.File {
	logsEndpoint := p.serverAddr + "/insert/jsonline"

	var b strings.Builder
	b.WriteString("# Generated by stroppy-cloud monitor phase\n")
	b.WriteString("data_dir: /var/lib/vector\n\n")

	// Sources: systemd journal everywhere, plus file tailers for DB engines
	// that prefer files over journald.
	b.WriteString("sources:\n")
	b.WriteString("  journald:\n")
	b.WriteString("    type: journald\n")
	b.WriteString("    current_boot_only: true\n")
	if p.dbKind == "postgres" {
		b.WriteString("  postgres_files:\n")
		b.WriteString("    type: file\n")
		b.WriteString("    include: ['/var/log/postgresql/*.log']\n")
		b.WriteString("    read_from: end\n")
	}
	if p.dbKind == "mysql" {
		b.WriteString("  mysql_files:\n")
		b.WriteString("    type: file\n")
		b.WriteString("    include: ['/var/log/mysql/*.log']\n")
		b.WriteString("    read_from: end\n")
	}

	b.WriteString("\ntransforms:\n")
	// Multiline join: DB engines emit multi-line records via journald / files;
	// reduce groups continuation lines back into the parent record.
	b.WriteString("  multiline_join:\n")
	inputs := "['journald'"
	if p.dbKind == "postgres" {
		inputs += ", 'postgres_files'"
	}
	if p.dbKind == "mysql" {
		inputs += ", 'mysql_files'"
	}
	inputs += "]"
	fmt.Fprintf(&b, "    inputs: %s\n", inputs)
	b.WriteString("    type: reduce\n")
	b.WriteString("    group_by: ['_SYSTEMD_UNIT', 'host', 'file']\n")
	b.WriteString("    starts_when: |\n")
	b.WriteString("      msg = \"\"\n")
	b.WriteString("      if is_string(.message) {\n")
	b.WriteString("        msg = string!(.message)\n")
	b.WriteString("      } else if is_string(.MESSAGE) {\n")
	b.WriteString("        msg = string!(.MESSAGE)\n")
	b.WriteString("      }\n")
	b.WriteString("      match(msg, r'^(\\d{4}-\\d{2}-\\d{2}|:[A-Z][A-Z0-9_]+\\s)')\n")
	b.WriteString("    merge_strategies:\n")
	b.WriteString("      message: concat_newline\n")
	b.WriteString("    expire_after_ms: 2000\n")
	b.WriteString("    flush_period_ms: 500\n")

	// Enrich: stamp run/machine identifiers and normalise the message + unit.
	b.WriteString("  enrich:\n")
	b.WriteString("    inputs: ['multiline_join']\n")
	b.WriteString("    type: remap\n")
	b.WriteString("    source: |\n")
	fmt.Fprintf(&b, "      .run_id = %q\n", p.runID)
	fmt.Fprintf(&b, "      .machine_id = %q\n", p.machineID)
	fmt.Fprintf(&b, "      .role = %q\n", p.role)
	b.WriteString("      .source = \"journald\"\n")
	b.WriteString("      if is_string(.source_type) && string!(.source_type) == \"file\" { .source = \"file\" }\n")
	b.WriteString("      if exists(.file) { .source = \"file\" }\n")
	b.WriteString("      .stream = \"stdout\"\n")
	b.WriteString("      .unit = \"\"\n")
	b.WriteString("      if is_string(._SYSTEMD_UNIT) {\n")
	b.WriteString("        .unit = ._SYSTEMD_UNIT\n")
	b.WriteString("      } else if is_string(.SYSTEMD_UNIT) {\n")
	b.WriteString("        .unit = .SYSTEMD_UNIT\n")
	b.WriteString("      } else if is_string(.file) {\n")
	b.WriteString("        .unit = .file\n")
	b.WriteString("      } else if is_string(.source_type) {\n")
	b.WriteString("        .unit = .source_type\n")
	b.WriteString("      }\n")
	b.WriteString("      if !exists(.timestamp) {\n")
	b.WriteString("        .timestamp = now()\n")
	b.WriteString("      }\n")
	b.WriteString("      if is_string(.message) { .message = .message } else if is_string(.MESSAGE) { .message = .MESSAGE } else { .message = encode_json(.) }\n")

	// HTTP sink → gateway /insert/jsonline → vmauth → vlinsert. AccountID 0.
	b.WriteString("\nsinks:\n")
	b.WriteString("  victorialogs:\n")
	b.WriteString("    type: http\n")
	b.WriteString("    inputs: ['enrich']\n")
	fmt.Fprintf(&b, "    uri: %q\n", logsEndpoint)
	b.WriteString("    method: post\n")
	b.WriteString("    encoding:\n")
	b.WriteString("      codec: json\n")
	b.WriteString("    framing:\n")
	b.WriteString("      method: newline_delimited\n")
	b.WriteString("    request:\n")
	b.WriteString("      headers:\n")
	if p.bearerToken != "" {
		fmt.Fprintf(&b, "        Authorization: %q\n", "Bearer "+p.bearerToken)
	}
	fmt.Fprintf(&b, "        AccountID: %q\n", fmt.Sprintf("%d", monitorAccountID))
	// VictoriaLogs field mapping (https://docs.victoriametrics.com/victorialogs/data-ingestion/).
	// Without these the events carry the log line in "message" with no "_msg",
	// so VictoriaLogs records the literal "missing _msg field" placeholder and
	// stamps every event with the ingest time instead of the real timestamp.
	b.WriteString("        VL-Msg-Field: \"message\"\n")
	b.WriteString("        VL-Time-Field: \"timestamp\"\n")
	b.WriteString("        VL-Stream-Fields: \"run_id,machine_id,role,unit,source\"\n")
	b.WriteString("    batch:\n")
	b.WriteString("      max_events: 1000\n")
	b.WriteString("      timeout_secs: 5\n")

	return &common.File{
		Info: &common.File_Info{
			Path:          "/etc/vector/vector.yaml",
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: b.String()},
	}
}

// startVectorScript starts the vector log shipper. Best-effort: vector may have
// failed to install (non-critical), so a missing binary must not fail deploy.
func startVectorScript() string {
	exec := "/usr/local/bin/vector --config /etc/vector/vector.yaml"
	return fmt.Sprintf(`mkdir -p /etc/vector /var/lib/vector
if command -v vector >/dev/null 2>&1; then
  systemctl stop %s 2>/dev/null || true
  systemd-run --unit=%s --collect %s
else
  echo "vector not installed (logs to VictoriaLogs disabled)"
fi
`, ShellQuote("stroppy-vector"), ShellQuote("stroppy-vector"), exec)
}
