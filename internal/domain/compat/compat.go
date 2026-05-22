// Package compat is the engine compatibility matrix + install recipes as DATA
// (H29: recipes are backend data, not hand-wired Go logic). It is proto-agnostic
// (string engine names) so it can later be sourced from a backend table/config; the
// planner bridges the Database.Kind enum to these names and applies shape-specific
// adjustments (e.g. patroni) on top of the looked-up Recipe.
package compat

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
		// config (cluster peers come from instance.peer in the yaml).
		StartScript: "install -d -o picodata -g picodata /var/lib/picodata 2>/dev/null || install -d /var/lib/picodata; " +
			"systemd-run --unit=stroppy-picodata --collect picodata run --config /etc/picodata/picodata.yaml --instance-dir /var/lib/picodata",
	},
	"cockroach/24.2": {
		PreInstall: []string{
			"apt-get update",
			"apt-get install -y curl ca-certificates tar",
			`curl -fsSL https://binaries.cockroachdb.com/cockroach-v24.2.0.linux-amd64.tgz -o /tmp/cockroach.tgz`,
			`tar -xzf /tmp/cockroach.tgz -C /tmp`,
			`install /tmp/cockroach-v24.2.0.linux-amd64/cockroach /usr/local/bin/cockroach`,
		},
		StartScript: `cockroach start-single-node --insecure --background --listen-addr=0.0.0.0:26257 --sql-addr=0.0.0.0:5432`,
	},
	"ydb/24.2": {
		PreInstall: []string{
			"apt-get update",
			"apt-get install -y curl ca-certificates tar",
			`curl -fsSL https://binaries.ydb.tech/release/24.2.7/ydbd-24.2.7-linux-amd64.tar.gz -o /tmp/ydbd.tgz`,
			`mkdir -p /opt/ydb && tar -xzf /tmp/ydbd.tgz -C /opt/ydb --strip-components=1`,
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
	Etcd = Recipe{
		PreInstall:  []string{"apt-get update", "apt-get install -y software-properties-common", "add-apt-repository -y universe", "apt-get update"},
		AptPackages: []string{"etcd-server", "etcd-client"}, ServiceName: "etcd",
	}
	HAProxy  = Recipe{PreInstall: []string{"apt-get update"}, AptPackages: []string{"haproxy"}, ServiceName: "haproxy"}
	ProxySQL = Recipe{
		PreInstall: []string{
			"apt-get update",
			`apt-get install -y curl ca-certificates gnupg lsb-release`,
			`install -d /etc/apt/keyrings`,
			`curl -fsSL https://repo.proxysql.com/ProxySQL/proxysql-2.x/repo_pub_key | gpg --dearmor -o /etc/apt/keyrings/proxysql.gpg`,
			`bash -c 'echo "deb [signed-by=/etc/apt/keyrings/proxysql.gpg] https://repo.proxysql.com/ProxySQL/proxysql-2.x/$(lsb_release -cs)/ ./" > /etc/apt/sources.list.d/proxysql.list'`,
			`apt-get update`,
		},
		AptPackages: []string{"proxysql"}, ServiceName: "proxysql",
	}
	Monitor = Recipe{PreInstall: []string{"apt-get update"}, AptPackages: []string{"prometheus-node-exporter"}, ServiceName: "prometheus-node-exporter"}
)

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
	`sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'`,
	"sh -c 'wget --quiet -O /etc/apt/trusted.gpg.d/pgdg.asc https://www.postgresql.org/media/keys/ACCC4CF8.asc'",
	"apt-get update",
}

func mysqlPreInstall(component string) []string {
	return []string{
		`apt-get update`,
		`apt-get install -y curl gnupg lsb-release ca-certificates`,
		`install -d /etc/apt/keyrings`,
		`curl -fsSL https://repo.mysql.com/RPM-GPG-KEY-mysql-2023 | gpg --dearmor -o /etc/apt/keyrings/mysql.gpg`,
		`bash -c 'echo "deb [signed-by=/etc/apt/keyrings/mysql.gpg] http://repo.mysql.com/apt/ubuntu/ $(lsb_release -cs) ` + component + `" > /etc/apt/sources.list.d/mysql.list'`,
		`apt-get update`,
	}
}

// mysqlTrustedPreInstall adds the mysql.com repo with [trusted=yes], bypassing the
// expired upstream Release-signing key (no current key exists). Use only for the
// versions absent from the Ubuntu archive (8.4 LTS).
func mysqlTrustedPreInstall(component string) []string {
	return []string{
		`apt-get update`,
		`apt-get install -y curl ca-certificates lsb-release`,
		`bash -c 'echo "deb [trusted=yes] http://repo.mysql.com/apt/ubuntu/ $(lsb_release -cs) ` + component + `" > /etc/apt/sources.list.d/mysql.list'`,
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
