package dag

import "strings"

import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"

// ── engine compatibility matrix + install recipes, as DATA ──────────────────────
//
// This file is the recipe DATA (formerly package compat, H29: recipes are backend
// data, not hand-wired Go logic). It is proto-agnostic (string engine names) so it
// can later be sourced from a backend table/config; recipe.go bridges the
// Database.Kind enum to these names and applies shape-specific adjustments (e.g.
// patroni) on top of the looked-up Recipe. Kept as a clearly-separated recipe-data
// file within the consolidated dag package.

// Recipe is the per-engine on-host install plan: shell repo/setup steps, apt
// packages, and either a systemd service to start or a start script.
type Recipe struct {
	PreInstall  []string
	AptPackages []string
	ServiceName string // apt engines: systemctl enable --now <ServiceName>
	StartScript string // binary engines (no apt unit): shell to start the node
}

// EngineVersion identifies one supported (engine, version) cell of the matrix.
type EngineVersion struct {
	Engine  string
	Version string
}

// engineRecipes is the compatibility matrix: supported (engine/version) -> recipe.
var engineRecipes = map[string]Recipe{
	"postgres/16": {PreInstall: pgPreInstall, AptPackages: []string{"postgresql-16", "postgresql-client-16"}, ServiceName: "postgresql"},
	"postgres/17": {PreInstall: pgPreInstall, AptPackages: []string{"postgresql-17", "postgresql-client-17"}, ServiceName: "postgresql"},
	// 8.0 ships in the Ubuntu archive (universe) — no third-party repo/key needed.
	"mysql/8.0": {PreInstall: ubuntuUniversePreInstall, AptPackages: []string{"mysql-server"}, ServiceName: "mysql"},
	// 8.4 (latest LTS) only exists in the upstream mysql.com APT repo, whose Release
	// is still signed by an expired GPG key (B7B3B788A8D3785C, expired 2025-10) —
	// apt rejects it. We pin the repo [trusted=yes] to install the latest LTS anyway
	// (benchmark target, not a security boundary).
	"mysql/8.4":     {PreInstall: mysqlTrustedPreInstall("mysql-8.4-lts"), AptPackages: []string{"mysql-server", "mysql-client"}, ServiceName: "mysql"},
	"mariadb/10.11": {PreInstall: mariadbPreInstall("10.11"), AptPackages: []string{"mariadb-server", "mariadb-client"}, ServiceName: "mariadb"},
	"mariadb/11.4":  {PreInstall: mariadbPreInstall("11.4"), AptPackages: []string{"mariadb-server", "mariadb-client"}, ServiceName: "mariadb"},
	"picodata/25.3": {
		PreInstall: []string{
			"apt-get update",
			"apt-get install -y curl gnupg ca-certificates lsb-release",
			`curl -fsSL https://download.picodata.io/tarantool-picodata/picodata.gpg.key | gpg --no-default-keyring --keyring gnupg-ring:/etc/apt/trusted.gpg.d/picodata.gpg --import && chmod 644 /etc/apt/trusted.gpg.d/picodata.gpg`,
			`echo "deb https://download.picodata.io/tarantool-picodata/ubuntu/ $(lsb_release -cs) main" > /etc/apt/sources.list.d/picodata.list`,
			"apt-get update",
		},
		AptPackages: []string{"picodata"},
		// picodata has no package systemd unit — run it via a transient unit with our
		// config (cluster peers come from instance.peer in the yaml). Idempotent across
		// the 60s agent-lease re-delivery + waits for the pg listener so the workload
		// node does not connect before 5432 is up.
		StartScript: strings.Join([]string{
			"install -d -o picodata -g picodata /var/lib/picodata 2>/dev/null || install -d /var/lib/picodata",
			// Substitute the real node IP into the advertise addresses (the pg driver's
			// discovery dials the advertised host, so it must be reachable, not 0.0.0.0).
			`sed -i "s/__SELF_IP__/$(hostname -i | awk '{print $1}')/g" /etc/picodata/picodata.yaml`,
			// Start only if not already up (idempotent across the 60s lease re-delivery).
			// PICODATA_ADMIN_PASSWORD sets the `admin` pg user's password (stroppy connects
			// as admin:T0psecret — see stroppyURL); without it pg auth fails.
			"if ! (echo > /dev/tcp/127.0.0.1/5432) 2>/dev/null; then",
			"  systemctl reset-failed stroppy-picodata >/dev/null 2>&1 || true",
			"  systemctl is-active --quiet stroppy-picodata || systemd-run --unit=stroppy-picodata --setenv=PICODATA_ADMIN_PASSWORD=T0psecret --collect picodata run --config /etc/picodata/picodata.yaml --instance-dir /var/lib/picodata",
			"  for i in $(seq 1 180); do (echo > /dev/tcp/127.0.0.1/5432) 2>/dev/null && break; sleep 1; done",
			"fi",
			"(echo > /dev/tcp/127.0.0.1/5432) 2>/dev/null || exit 1",
			// Raise the SQL vdbe opcode limit (default 45000) so the tpcc consistency-check
			// aggregations (MAX/SUM/GROUP BY over loaded data) don't hit "Reached a limit
			// on max executed vdbe opcodes". RETRY until it actually succeeds (rc=0): on a
			// multi-node cluster the pg listener (5432) comes up before the raft governor
			// has settled, so an early ALTER SYSTEM errors out — the old `|| true` swallowed
			// that and left the limit at 45000, failing the workload on the storage
			// replicasets. The setting is cluster-wide + idempotent; the node is not
			// COMPLETED (so the FLOW workload cannot start) until it is verifiably applied.
			"for i in $(seq 1 90); do",
			`  echo 'ALTER SYSTEM SET "sql_vdbe_opcode_max" TO 1073741824;' | picodata admin /var/lib/picodata/admin.sock >/dev/null 2>&1 && exit 0`,
			"  sleep 2",
			"done",
			"exit 1",
		}, "\n"),
	},
	"cockroach/24.2": {
		PreInstall: []string{
			"apt-get update",
			"apt-get install -y curl ca-certificates tar",
			cockroachInstallScript("24.2"),
		},
	},
	"ydb/24.2": {
		PreInstall: []string{
			"apt-get update",
			"apt-get install -y curl ca-certificates tar",
			// ydbd is a ~176MB tarball: fetch it through the server binary cache (bincache
			// "ydb" upstream), NOT direct — the server downloads it ONCE and streams to every
			// agent over the LAN, so 3 cluster nodes don't each hammer binaries.ydb.tech (a
			// dropped concurrent download previously left a node without the binary). Same
			// path local + cloud. Idempotent: skip if already extracted.
			`test -x /opt/ydb/bin/ydbd || curl -fsSL --retry 8 --retry-delay 3 "${STROPPY_SERVER_ADDR}/binary/ydb/24.2.7/ydbd-24.2.7-linux-amd64.tar.gz" -o /tmp/ydbd.tgz`,
			`test -x /opt/ydb/bin/ydbd || (mkdir -p /opt/ydb && tar -xzf /tmp/ydbd.tgz -C /opt/ydb --strip-components=1)`,
			`ln -sf /opt/ydb/bin/ydbd /usr/local/bin/ydbd`,
		},
		// Start + cluster bootstrap are render COMMAND items (config.yaml + ydbd
		// storage start + readiness), so no service/startScript here.
	},
}

// defaultVersions picks a version when the preset omits one.
var defaultVersions = map[string]string{
	"postgres": "16", "mysql": "8.4", "mariadb": "11.4",
	"picodata": "25.3", "cockroach": "24.2", "ydb": "24.2",
}

var cockroachVersionMap = map[string]string{
	"24.2": "24.2.10",
}

func resolveCockroachVersion(v string) string {
	if v == "" {
		return cockroachVersionMap["24.2"]
	}
	if full, ok := cockroachVersionMap[v]; ok {
		return full
	}
	return v
}

func cockroachInstallScript(version string) string {
	full := resolveCockroachVersion(version)
	file := "cockroach-v" + full + ".linux-amd64.tgz"
	archive := "cockroach-v" + full + ".linux-amd64/cockroach"
	return strings.Join([]string{
		"set -e",
		fetchBinary("cockroach", full, file, archive, "cockroach"),
	}, "\n")
}

// EngineRecipe returns the install recipe for an (engine, version); version "" uses
// the engine default. The bool is false for an unsupported cell.
func EngineRecipe(engine, version string) (Recipe, bool) {
	if version == "" {
		version = defaultVersions[engine]
	}
	r, ok := engineRecipes[engine+"/"+version]
	if !ok {
		// fall back to the engine default version cell.
		r, ok = engineRecipes[engine+"/"+defaultVersions[engine]]
	}
	return r, ok
}

// DefaultVersion is the default version for an engine ("" if unknown).
func DefaultVersion(engine string) string { return defaultVersions[engine] }

// Supported lists every (engine, version) cell of the matrix.
func Supported() []EngineVersion {
	out := make([]EngineVersion, 0, len(engineRecipes))
	for k := range engineRecipes {
		for i := 0; i < len(k); i++ {
			if k[i] == '/' {
				out = append(out, EngineVersion{Engine: k[:i], Version: k[i+1:]})
				break
			}
		}
	}
	return out
}

// Non-DATABASE component recipes (etcd / proxies / monitoring) of an emergent HA or
// cluster topology. Configs are data (component.config); these carry install+service.
var (
	recipeEtcd = Recipe{
		PreInstall:  []string{"apt-get update", "apt-get install -y software-properties-common", "add-apt-repository -y universe", "apt-get update"},
		AptPackages: []string{"etcd-server", "etcd-client"}, ServiceName: "etcd",
	}
	recipeHAProxy  = Recipe{PreInstall: []string{"apt-get update"}, AptPackages: []string{"haproxy"}, ServiceName: "haproxy"}
	recipeProxySQL = Recipe{
		PreInstall: []string{
			"apt-get update",
			`apt-get install -y curl ca-certificates gnupg lsb-release`,
			`install -d /etc/apt/keyrings`,
			// proxysql-2.x is gone from the repo (404); the key is now version-agnostic
			// and the deb path is proxysql-2.7.x. Stale URLs failed px*.pre_install.
			// --retry: repo.proxysql.com is https-direct (uncached) and flakes — a dropped
			// key fetch failed px*.pre_install_3 and skipped the whole group/replica run.
			`curl -fsSL --retry 5 --retry-delay 3 https://repo.proxysql.com/ProxySQL/repo_pub_key | gpg --dearmor -o /etc/apt/keyrings/proxysql.gpg`,
			`bash -c 'echo "deb [signed-by=/etc/apt/keyrings/proxysql.gpg] https://repo.proxysql.com/ProxySQL/proxysql-2.7.x/$(lsb_release -cs)/ ./" > /etc/apt/sources.list.d/proxysql.list'`,
			`apt-get update`,
		},
		AptPackages: []string{"proxysql"}, ServiceName: "proxysql",
	}
	recipeMonitor = Recipe{PreInstall: []string{"apt-get update"}, AptPackages: []string{"prometheus-node-exporter"}, ServiceName: "prometheus-node-exporter"}
)

// ── monitoring pipeline DATA ───────────────────────────────────────────────────
//
// The full metrics pipeline (ported from internal/old/domain/agent/executor.go):
// every machine runs node_exporter (host metrics) + vmagent (scrapes local
// exporters and remote_writes to the server's VictoriaMetrics ingest); DB
// machines additionally run an engine-specific exporter (postgres/mysql) or, for
// engines that expose Prometheus natively (cockroach/ydb/picodata), are scraped
// directly on the engine's metrics port. These constants are recipe DATA so the
// versions/ports/URLs live next to the other engine recipes, not hand-wired in
// the builder.

const (
	// nodeExporterPort is node_exporter's default listen port (host metrics).
	nodeExporterPort = "9100"
	// Binary versions fetched from the SERVER cache (bincache) — never apt/github
	// directly, so the path is identical for local Docker and Yandex Cloud VMs.
	nodeExporterVersion     = "1.9.1"
	postgresExporterVersion = "0.15.0"
	mysqldExporterVersion   = "0.15.1"
	// vmagentVersion mirrors the old executor's default vmutils release.
	vmagentVersion = "1.139.0"
	// vectorVersion is the log shipper (journald + DB log files → VictoriaLogs).
	vectorVersion = "0.43.1"
	// remoteWritePath is the server route reverse-proxied to VictoriaMetrics ingest.
	// The cluster's vminsert is reached via vmauth's per-tenant /insert/<acct>/prometheus
	// route (acct 0 = default). STROPPY_SERVER_ADDR is expanded by the agent at exec time.
	remoteWritePath = "/vm/insert/0/prometheus/api/v1/write"
	// vlInsertPath is the server route reverse-proxied to VictoriaLogs ingest.
	vlInsertPath = "/vl/insert/jsonline?_stream_fields=dag_id,machine_id,unit&_msg_field=message&_time_field=timestamp"
)

// dbExporter describes how a DATABASE engine's metrics are collected:
//   - aptPackage/binary != "" → a separate exporter process is installed and
//     scraped on scrapePort;
//   - native == true → the engine itself exposes Prometheus metrics; no exporter,
//     vmagent just scrapes scrapePort directly.
type dbExporter struct {
	job        string // vmagent scrape job_name
	scrapePort string // localhost:<port> vmagent scrapes
	metricPath string // metrics_path (default /metrics when empty)
	native     bool   // engine exposes Prometheus natively (no separate exporter)

	// Separate-exporter install (only when native == false). The binary is fetched
	// from the server cache (bincache name/version/archive file), extracted, and run
	// as a systemd unit — no apt, so it works identically on a fresh YC VM.
	binName     string // bincache binary name (e.g. "postgres_exporter")
	binVersion  string // bincache version
	binFile     string // release asset filename
	binArchive  string // path of the binary inside the extracted archive
	serviceName string // systemd unit name for the exporter
	// dataSourceEnv is the env line baked into the exporter's systemd unit so it
	// connects to the LOCAL database (e.g. postgres_exporter DATA_SOURCE_NAME).
	dataSourceEnv string
	execStart     string // ExecStart= line for the exporter systemd unit
}

// dbExporterFor maps a database engine to its metrics-collection strategy. Engines
// not in the map (or KIND_UNSPECIFIED) yield ok=false → no DB exporter / scrape.
func dbExporterFor(kind domain.Database_Kind) (dbExporter, bool) {
	switch kind {
	case domain.Database_KIND_POSTGRES:
		// prometheus-postgres-exporter ships in the Ubuntu archive (universe);
		// connects to the LOCAL postgres over the unix-socket-equivalent TCP loopback.
		return dbExporter{
			job:           "postgres",
			scrapePort:    "9187",
			binName:       "postgres_exporter",
			binVersion:    postgresExporterVersion,
			binFile:       "postgres_exporter-" + postgresExporterVersion + ".linux-amd64.tar.gz",
			binArchive:    "postgres_exporter-" + postgresExporterVersion + ".linux-amd64/postgres_exporter",
			serviceName:   "stroppy-postgres-exporter",
			dataSourceEnv: `Environment=DATA_SOURCE_NAME=postgresql://postgres@localhost:5432/postgres?sslmode=disable`,
			execStart:     "/usr/local/bin/postgres_exporter",
		}, true
	case domain.Database_KIND_MYSQL, domain.Database_KIND_MARIADB:
		// prometheus-mysqld-exporter ships in the Ubuntu archive (universe);
		// reads creds from DATA_SOURCE_NAME pointing at the LOCAL mysql/mariadb.
		return dbExporter{
			job:           "mysql",
			scrapePort:    "9104",
			binName:       "mysqld_exporter",
			binVersion:    mysqldExporterVersion,
			binFile:       "mysqld_exporter-" + mysqldExporterVersion + ".linux-amd64.tar.gz",
			binArchive:    "mysqld_exporter-" + mysqldExporterVersion + ".linux-amd64/mysqld_exporter",
			serviceName:   "stroppy-mysqld-exporter",
			dataSourceEnv: `Environment=DATA_SOURCE_NAME=exporter:exporter@tcp(localhost:3306)/`,
			execStart:     "/usr/local/bin/mysqld_exporter",
		}, true
	case domain.Database_KIND_COCKROACH:
		// CockroachDB exposes Prometheus metrics natively on its HTTP port (8080)
		// at /_status/vars — no separate exporter.
		return dbExporter{job: "cockroach", scrapePort: "8080", metricPath: "/_status/vars", native: true}, true
	case domain.Database_KIND_YDB:
		// YDB exposes counters natively on the static node's mon port (8765).
		return dbExporter{job: "ydb", scrapePort: "8765", metricPath: "/counters/counters=ydb/prometheus", native: true}, true
	case domain.Database_KIND_PICODATA:
		// Picodata exposes Prometheus metrics natively on its HTTP port (8081).
		return dbExporter{job: "picodata", scrapePort: "8081", native: true}, true
	default:
		return dbExporter{}, false
	}
}

// ubuntuUniversePreInstall refreshes the package lists + enables universe (a minimal
// base image ships neither) for archive packages like mysql-server.
var ubuntuUniversePreInstall = []string{
	"apt-get update",
	"apt-get install -y software-properties-common",
	"add-apt-repository -y universe",
	"apt-get update",
}

var pgPreInstall = []string{
	// A minimal base image lacks wget/gnupg/lsb-release — install before pgdg.
	"apt-get update",
	"apt-get install -y wget gnupg lsb-release ca-certificates",
	// HTTPS (direct, bypasses the server apt cache): apt.postgresql.org is a Fastly
	// CDN redirector — apt-cacher-ng's range-resume on its volatile InRelease produces
	// stale partials that 503 "unexpected range". Same https-direct treatment as the
	// other 3rd-party DB repos (mariadb/proxysql/picodata); the cacheable ubuntu base
	// archive (direct mirror) stays http-cached where caching gives the most value.
	`sh -c 'echo "deb https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'`,
	"sh -c 'wget --quiet -O /etc/apt/trusted.gpg.d/pgdg.asc https://www.postgresql.org/media/keys/ACCC4CF8.asc'",
	"apt-get update",
}

// mysqlTrustedPreInstall adds the mysql.com repo with [trusted=yes], bypassing the
// expired upstream Release-signing key (no current key exists). Use only for the
// versions absent from the Ubuntu archive (8.4 LTS).
func mysqlTrustedPreInstall(component string) []string {
	return []string{
		`apt-get update`,
		`apt-get install -y curl ca-certificates lsb-release`,
		// HTTPS (direct, bypasses the apt cache): repo.mysql.com is a CDN redirector;
		// apt-cacher-ng range-resume on its volatile index can 503 "unexpected range".
		`bash -c 'echo "deb [trusted=yes] https://repo.mysql.com/apt/ubuntu/ $(lsb_release -cs) ` + component + `" > /etc/apt/sources.list.d/mysql.list'`,
		`apt-get update`,
	}
}

func mariadbPreInstall(version string) []string {
	return []string{
		`apt-get update`,
		`apt-get install -y curl ca-certificates`,
		`curl -fsSL https://r.mariadb.com/downloads/mariadb_repo_setup -o /tmp/mariadb_repo_setup`,
		`bash /tmp/mariadb_repo_setup --mariadb-server-version=` + version,
		`apt-get update`,
	}
}
