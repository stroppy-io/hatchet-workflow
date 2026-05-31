package run

import (
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// isYDBCombined reports whether the run is a YDB combined topology
// (storage VMs also host dynamic nodes — `topology.YDB.Database == nil`).
func isYDBCombined(db types.DatabaseConfig) bool {
	return db.Kind == types.DatabaseYDB && db.YDB != nil && db.YDB.Database == nil
}

// binURL composes the download URL for a monitoring binary. All artifacts are
// fetched through the cloud server's /api/binaries proxy instead of github
// directly: github SSL handshakes from Yandex Cloud are flaky, and one cold
// download per artifact warms the server cache for the whole fleet. When no
// server address is known (CLI / direct mode) we fall back to upstream.
func binURL(serverAddr, name, ver, file string) string {
	base := strings.TrimRight(serverAddr, "/")
	if base == "" {
		switch name {
		case "node_exporter":
			return fmt.Sprintf("https://github.com/prometheus/node_exporter/releases/download/v%s/%s", ver, file)
		case "mysqld_exporter":
			return fmt.Sprintf("https://github.com/prometheus/mysqld_exporter/releases/download/v%s/%s", ver, file)
		case "postgres_exporter":
			return fmt.Sprintf("https://github.com/prometheus-community/postgres_exporter/releases/download/v%s/%s", ver, file)
		case "vector":
			return fmt.Sprintf("https://packages.timber.io/vector/%s/%s", ver, file)
		case "vmagent":
			return fmt.Sprintf("https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v%s/%s", ver, file)
		case "stroppy":
			return fmt.Sprintf("https://github.com/stroppy-io/stroppy/releases/download/v%s/%s", ver, file)
		}
		return ""
	}
	return fmt.Sprintf("%s/api/binaries/%s/%s/%s", base, name, ver, file)
}

// isDatabaseMachine mirrors the old agent's machineID role detection. The
// machineID is "<runID>-<role>-<i>", identical to target.ID, so the server can
// reproduce the same per-role decisions without the agent knowing its role.
func isDatabaseMachine(machineID string) bool { return strings.Contains(machineID, "-database-") }

type monitorInstallTask struct {
	client     CommandSink
	state      *State
	dbKind     types.DatabaseKind
	serverAddr string
}

func (t *monitorInstallTask) Execute(nc *NodeContext) error {
	allTargets := t.state.AllTargets()
	nc.Log().Info("installing monitoring exporters on all machines", zap.Int("count", len(allTargets)))

	mon := types.DefaultMonitoring()
	dbKind := string(t.dbKind)

	return sendPerTarget(nc, t.client, allTargets, func(target agent.Target) []agent.Command {
		machineID := target.ID
		var cmds []agent.Command

		// node_exporter on ALL machines (skip if pre-installed in agent image).
		neVer := mon.NodeExporterVersion
		neFile := fmt.Sprintf("node_exporter-%s.linux-amd64.tar.gz", neVer)
		cmds = append(cmds, runCmd("install_monitor", fmt.Sprintf(
			`which node_exporter >/dev/null 2>&1 || { `+
				`curl -fsSL %s %q -o /tmp/node_exporter.tar.gz && `+
				`tar xzf /tmp/node_exporter.tar.gz -C /tmp && `+
				`cp /tmp/node_exporter-%s.linux-amd64/node_exporter /usr/local/bin/node_exporter && `+
				`chmod +x /usr/local/bin/node_exporter && `+
				`rm -rf /tmp/node_exporter*; }`,
			curlOpts, binURL(t.serverAddr, "node_exporter", neVer, neFile), neVer)))

		// mysqld_exporter on database machines (MySQL only).
		if isDatabaseMachine(machineID) && dbKind == "mysql" {
			meVer := "0.19.0"
			meFile := fmt.Sprintf("mysqld_exporter-%s.linux-amd64.tar.gz", meVer)
			cmds = append(cmds, runCmd("install_monitor", fmt.Sprintf(
				`which mysqld_exporter >/dev/null 2>&1 || { `+
					`curl -fsSL %s %q -o /tmp/mysqld_exporter.tar.gz && `+
					`tar xzf /tmp/mysqld_exporter.tar.gz -C /tmp && `+
					`cp /tmp/mysqld_exporter-%s.linux-amd64/mysqld_exporter /usr/local/bin/mysqld_exporter && `+
					`chmod +x /usr/local/bin/mysqld_exporter && `+
					`rm -rf /tmp/mysqld_exporter*; }`,
				curlOpts, binURL(t.serverAddr, "mysqld_exporter", meVer, meFile), meVer)))
		}

		// postgres_exporter on database machines (postgres only).
		if isDatabaseMachine(machineID) && dbKind == "postgres" {
			peVer := mon.PostgresExporterVersion
			peFile := fmt.Sprintf("postgres_exporter-%s.linux-amd64.tar.gz", peVer)
			cmds = append(cmds, runCmd("install_monitor", fmt.Sprintf(
				`curl -fsSL %s %q -o /tmp/postgres_exporter.tar.gz && `+
					`tar xzf /tmp/postgres_exporter.tar.gz -C /tmp && `+
					`cp /tmp/postgres_exporter-%s.linux-amd64/postgres_exporter /usr/local/bin/postgres_exporter && `+
					`chmod +x /usr/local/bin/postgres_exporter && `+
					`rm -rf /tmp/postgres_exporter*`,
				curlOpts, binURL(t.serverAddr, "postgres_exporter", peVer, peFile), peVer)))
		}

		// vector on every machine (best-effort: log shipping is non-critical).
		vecVer := "0.43.1"
		vecFile := fmt.Sprintf("vector-%s-x86_64-unknown-linux-musl.tar.gz", vecVer)
		cmds = append(cmds, runCmd("install_monitor", fmt.Sprintf(
			`which vector >/dev/null 2>&1 || { `+
				`curl -fsSL %s %q -o /tmp/vector.tar.gz && `+
				`tar xzf /tmp/vector.tar.gz -C /tmp && `+
				`cp /tmp/vector-x86_64-unknown-linux-musl/bin/vector /usr/local/bin/vector && `+
				`chmod +x /usr/local/bin/vector && `+
				`rm -rf /tmp/vector*; } || echo "install vector failed (logs to VictoriaLogs disabled)"`,
			curlOpts, binURL(t.serverAddr, "vector", vecVer, vecFile))))

		// vmagent on every machine (skip if pre-installed in agent image).
		vaVer := mon.VmagentVersion
		if vaVer == "" {
			vaVer = "1.139.0"
		}
		vaFile := fmt.Sprintf("vmutils-linux-amd64-v%s.tar.gz", vaVer)
		cmds = append(cmds, runCmd("install_monitor", fmt.Sprintf(
			`which vmagent >/dev/null 2>&1 || { `+
				`curl -fsSL %s %q -o /tmp/vmutils.tar.gz && `+
				`tar xzf /tmp/vmutils.tar.gz -C /tmp && `+
				`cp /tmp/vmagent-prod /usr/local/bin/vmagent && `+
				`chmod +x /usr/local/bin/vmagent && `+
				`rm -rf /tmp/vmutils* /tmp/vmagent* /tmp/vmalert* /tmp/vmauth* /tmp/vmbackup* /tmp/vmrestore*; }`,
			curlOpts, binURL(t.serverAddr, "vmagent", vaVer, vaFile))))

		return cmds
	})
}

type monitorConfigTask struct {
	client          CommandSink
	state           *State
	monitor         types.MonitorConfig
	runID           string
	dbKind          types.DatabaseKind
	ydbCombined     bool
	monitoringURL   string
	monitoringToken string
	accountID       int32
}

func (t *monitorConfigTask) Execute(nc *NodeContext) error {
	allTargets := t.state.AllTargets()
	nc.Log().Info("configuring monitoring on all machines", zap.Int("count", len(allTargets)))

	// VictoriaMetrics remote_write endpoint -- derived from monitoringURL + accountID.
	metricsEndpoint := t.monitor.MetricsEndpoint
	if metricsEndpoint == "" && t.monitoringURL != "" {
		metricsEndpoint = fmt.Sprintf("%s/insert/%d/prometheus/api/v1/write", t.monitoringURL, t.accountID)
	}

	// VictoriaLogs ingest endpoint. Vmauth routes /insert/jsonline → vlinsert
	// (see deployments/vmauth/config.yml). Stream fields key the log stream;
	// _msg_field tells VL which JSON key holds the actual message body.
	logsEndpoint := t.monitor.LogsEndpoint
	if logsEndpoint == "" && t.monitoringURL != "" {
		logsEndpoint = t.monitoringURL + "/insert/jsonline?_stream_fields=run_id,machine_id,role,unit&_msg_field=message&_time_field=timestamp"
	}

	dbKind := string(t.dbKind)

	return sendPerTarget(nc, t.client, allTargets, func(target agent.Target) []agent.Command {
		machineID := target.ID
		var cmds []agent.Command

		// node_exporter on EVERY machine.
		cmds = append(cmds, startDaemonCmd("config_monitor", "node_exporter", "/usr/local/bin/node_exporter", nil, nil))

		// postgres_exporter on database machines (connects to LOCAL postgres).
		if isDatabaseMachine(machineID) && dbKind == "postgres" {
			cmds = append(cmds, startDaemonCmd("config_monitor", "postgres_exporter", "/usr/local/bin/postgres_exporter", nil,
				map[string]string{"DATA_SOURCE_NAME": "postgresql://postgres@localhost:5432/postgres?sslmode=disable"}))
		}

		// mysqld_exporter on database machines (connects to LOCAL mysql).
		if isDatabaseMachine(machineID) && dbKind == "mysql" {
			cmds = append(cmds, runCmd("config_monitor",
				`mysql -h 127.0.0.1 -u root -e "SET sql_log_bin=0; CREATE USER IF NOT EXISTS 'exporter'@'localhost' IDENTIFIED BY 'exporter' WITH MAX_USER_CONNECTIONS 3; GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'exporter'@'localhost'; FLUSH PRIVILEGES; SET sql_log_bin=1;" 2>/dev/null || true`))
			cmds = append(cmds, startDaemonCmd("config_monitor", "mysqld_exporter", "/usr/local/bin/mysqld_exporter",
				[]string{"--mysqld.address=localhost:3306", "--mysqld.username=exporter"},
				map[string]string{"MYSQLD_EXPORTER_PASSWORD": "exporter"}))
		}

		// vmagent on every machine (scrapes local exporters, pushes to VictoriaMetrics).
		scrapeCfg := buildVmagentScrapeConfig(machineID, dbKind, t.runID, t.ydbCombined)
		cmds = append(cmds, writeFile("config_monitor", "/etc/vmagent/scrape.yml", scrapeCfg))
		cmds = append(cmds, runCmd("config_monitor", "mkdir -p /var/lib/vmagent"))
		vmagentArgs := []string{
			"-promscrape.config=/etc/vmagent/scrape.yml",
			"-remoteWrite.url=" + metricsEndpoint,
			"-remoteWrite.tmpDataPath=/var/lib/vmagent",
		}
		if t.monitoringToken != "" {
			vmagentArgs = append(vmagentArgs, "-remoteWrite.bearerToken="+t.monitoringToken)
		}
		cmds = append(cmds, startDaemonCmd("config_monitor", "vmagent", "/usr/local/bin/vmagent", vmagentArgs, nil))

		// vector log shipper (journald + DB log files → VictoriaLogs). Vector is
		// best-effort (the install step swallows download failures), so start it
		// only when the binary is actually present — never fail the run for a
		// missing log shipper.
		if logsEndpoint != "" {
			vecCfg := buildVectorConfig(machineID, dbKind, t.runID, logsEndpoint, t.monitoringToken, t.accountID)
			cmds = append(cmds, writeFile("config_monitor", "/etc/vector/vector.yaml", vecCfg))
			cmds = append(cmds, runCmd("config_monitor",
				`mkdir -p /var/lib/vector; if [ -x /usr/local/bin/vector ]; then `+
					`systemctl reset-failed vector 2>/dev/null; `+
					`systemd-run --unit=vector -- /usr/local/bin/vector --config /etc/vector/vector.yaml; `+
					`else echo "vector not installed; log shipping to VictoriaLogs disabled"; fi`))
		}

		return cmds
	})
}

// buildVmagentScrapeConfig renders the per-machine vmagent scrape config. Ported
// verbatim from the old agent configMonitor. machineID role detection decides
// which DB exporter (and which YDB counter groups) to scrape.
func buildVmagentScrapeConfig(machineID, dbKind, runID string, ydbCombined bool) string {
	var confBuf strings.Builder
	confBuf.WriteString("# Generated by stroppy-agent\n")
	confBuf.WriteString("global:\n  scrape_interval: 5s\n")

	fmt.Fprintf(&confBuf, "  external_labels:\n    stroppy_machine_id: '%s'\n", machineID)
	if runID != "" {
		fmt.Fprintf(&confBuf, "    stroppy_run_id: '%s'\n", runID)
	}

	confBuf.WriteString("\nscrape_configs:\n")

	// node_exporter on localhost.
	confBuf.WriteString("  - job_name: node\n    static_configs:\n      - targets: ['localhost:9100']\n")

	// DB exporter on localhost (only on database machines).
	isYDBStorage := strings.Contains(machineID, "-ydb-storage-")
	isYDBDatabase := strings.Contains(machineID, "-ydb-database-")
	isCombinedDB := strings.Contains(machineID, "-database-") && !isYDBStorage && !isYDBDatabase
	if isCombinedDB || isYDBStorage || isYDBDatabase {
		switch dbKind {
		case "postgres":
			if isCombinedDB {
				confBuf.WriteString("  - job_name: postgres\n    static_configs:\n      - targets: ['localhost:9187']\n")
			}
		case "mysql":
			if isCombinedDB {
				confBuf.WriteString("  - job_name: mysql\n    static_configs:\n      - targets: ['localhost:9104']\n")
			}
		case "picodata":
			if isCombinedDB {
				confBuf.WriteString("  - job_name: picodata\n    static_configs:\n      - targets: ['localhost:8081']\n    metrics_path: /metrics\n")
			}
		case "ydb":
			// YDB exposes counters at /counters/counters=<group>/prometheus. Metric names are
			// returned WITHOUT a group prefix (e.g. `DataShard_RowReads`), but the official
			// ydb-platform Grafana dashboards expect them prefixed (`tablets_DataShard_RowReads`).
			// Mirror the upstream Helm chart behaviour: scrape every counter group from both
			// static (:8765) and dynamic (:8766) nodes and prepend `<group>_` to __name__ via
			// metric_relabel_configs.
			type ydbCounter struct {
				name string // counter group identifier and metric prefix
				path string // optional custom metrics_path (default /counters/counters=<name>/prometheus)
				role string // "static", "dynamic", or "" for both
			}
			ydbCounters := []ydbCounter{
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
			type ydbRole struct{ name, port, container string }
			allRoles := []ydbRole{
				{"static", "8765", "ydb-static"},
				{"dynamic", "8766", "ydb-dynamic"},
			}
			var roles []ydbRole
			switch {
			case isYDBStorage && ydbCombined:
				roles = allRoles // combined on YC: both daemons on storage VM
			case isYDBStorage:
				roles = allRoles[:1] // static only
			case isYDBDatabase:
				roles = allRoles[1:] // dynamic only
			default:
				roles = allRoles // docker combined
			}
			for _, role := range roles {
				for _, c := range ydbCounters {
					if c.role != "" && c.role != role.name {
						continue
					}
					path := c.path
					if path == "" {
						path = fmt.Sprintf("/counters/counters=%s/prometheus", c.name)
					}
					fmt.Fprintf(&confBuf,
						"  - job_name: ydb_%s_%s\n"+
							"    metrics_path: %s\n"+
							"    static_configs:\n"+
							"    - targets: ['localhost:%s']\n"+
							"      labels:\n"+
							"        container: %s\n"+
							"        counter: %s\n"+
							"    metric_relabel_configs:\n"+
							"    - source_labels: [__name__]\n"+
							"      regex: (.*)\n"+
							"      target_label: __name__\n"+
							"      replacement: %s_$1\n",
						c.name, role.name, path, role.port, role.container, c.name, c.name,
					)
				}
			}
		}
	}

	return confBuf.String()
}

// buildVectorConfig renders the Vector config that tails journald (and
// DB-specific log files) and pushes JSON-line entries to VictoriaLogs through
// vmauth. Ported verbatim from the old agent.
func buildVectorConfig(machineID, dbKind, runID, logsEndpoint, bearerToken string, accountID int32) string {
	// Derive role from machineID convention "<runID>-<role>-<i>".
	role := "unknown"
	switch {
	case strings.Contains(machineID, "-ydb-storage-"):
		role = "ydb-storage"
	case strings.Contains(machineID, "-ydb-database-"):
		role = "ydb-database"
	case strings.Contains(machineID, "-database-"):
		role = "database"
	case strings.Contains(machineID, "-stroppy-"):
		role = "stroppy"
	case strings.Contains(machineID, "-proxy-"):
		role = "proxy"
	case strings.Contains(machineID, "-monitor-"):
		role = "monitor"
	case strings.Contains(machineID, "-etcd-"):
		role = "etcd"
	}

	var b strings.Builder
	b.WriteString("# Generated by stroppy-agent\n")
	b.WriteString("data_dir: /var/lib/vector\n\n")

	b.WriteString("sources:\n")
	b.WriteString("  journald:\n")
	b.WriteString("    type: journald\n")
	b.WriteString("    current_boot_only: true\n")

	if dbKind == "postgres" {
		b.WriteString("  postgres_files:\n")
		b.WriteString("    type: file\n")
		b.WriteString("    include: ['/var/log/postgresql/*.log']\n")
		b.WriteString("    read_from: end\n")
	}
	if dbKind == "mysql" || dbKind == "mariadb" {
		b.WriteString("  mysql_files:\n")
		b.WriteString("    type: file\n")
		b.WriteString("    include: ['/var/log/mysql/*.log']\n")
		b.WriteString("    read_from: end\n")
	}

	b.WriteString("\ntransforms:\n")
	b.WriteString("  multiline_join:\n")
	multilineInputs := "['journald'"
	if dbKind == "postgres" {
		multilineInputs += ", 'postgres_files'"
	}
	if dbKind == "mysql" || dbKind == "mariadb" {
		multilineInputs += ", 'mysql_files'"
	}
	multilineInputs += "]"
	fmt.Fprintf(&b, "    inputs: %s\n", multilineInputs)
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

	b.WriteString("  enrich:\n")
	b.WriteString("    inputs: ['multiline_join']\n")
	b.WriteString("    type: remap\n")
	b.WriteString("    source: |\n")
	fmt.Fprintf(&b, "      .run_id = %q\n", runID)
	fmt.Fprintf(&b, "      .machine_id = %q\n", machineID)
	fmt.Fprintf(&b, "      .role = %q\n", role)
	b.WriteString("      .unit = \"\"\n")
	b.WriteString("      if is_string(.SYSTEMD_UNIT) {\n")
	b.WriteString("        .unit = .SYSTEMD_UNIT\n")
	b.WriteString("      } else if is_string(.source_type) {\n")
	b.WriteString("        .unit = .source_type\n")
	b.WriteString("      }\n")
	b.WriteString("      if !exists(.timestamp) {\n")
	b.WriteString("        .timestamp = now()\n")
	b.WriteString("      }\n")
	b.WriteString("      if is_string(.message) { .message = .message } else if is_string(.MESSAGE) { .message = .MESSAGE } else { .message = encode_json(.) }\n")

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
	if bearerToken != "" || accountID > 0 {
		b.WriteString("    request:\n")
		b.WriteString("      headers:\n")
		if bearerToken != "" {
			fmt.Fprintf(&b, "        Authorization: %q\n", "Bearer "+bearerToken)
		}
		if accountID > 0 {
			fmt.Fprintf(&b, "        AccountID: %q\n", fmt.Sprintf("%d", accountID))
		}
	}
	b.WriteString("    batch:\n")
	b.WriteString("      max_events: 1000\n")
	b.WriteString("      timeout_secs: 5\n")

	return b.String()
}
