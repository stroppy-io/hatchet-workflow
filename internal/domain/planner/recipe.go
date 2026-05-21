package planner

import (
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

// stroppyDBHostToken is the late-binding hole in the run_stroppy command,
// resolved to the database component's private ip at the plan->execute seam.
const stroppyDBHostToken = "__STROPPY_DB_HOST__"

// recipe is the per-engine install plan: shell repo/setup steps, apt packages, and
// the systemd service to start. Recipes are planner DATA (H29) — recast of the old
// types.BuiltinPackages.
//
// TODO(planner): move recipes to backend data (compat matrix). ydb/cockroach ship
// no apt package (binary download) — not covered yet.
type recipe struct {
	preInstall  []string
	aptPackages []string
	serviceName string // apt engines: systemctl enable --now <serviceName>
	startScript string // binary engines (no apt/systemd unit): shell to start the node
}

var recipes = map[string]recipe{
	"postgres/16": {preInstall: pgPreInstall, aptPackages: []string{"postgresql-16", "postgresql-client-16"}, serviceName: "postgresql"},
	"postgres/17": {preInstall: pgPreInstall, aptPackages: []string{"postgresql-17", "postgresql-client-17"}, serviceName: "postgresql"},
	// 8.0 ships in the Ubuntu archive (universe) — no third-party repo/key needed.
	"mysql/8.0": {preInstall: ubuntuUniversePreInstall, aptPackages: []string{"mysql-server"}, serviceName: "mysql"},
	// 8.4 only exists in the upstream mysql.com APT repo (note: its GPG key has
	// expired upstream — apt-get update can fail until mysql publishes a new key).
	"mysql/8.4":     {preInstall: mysqlPreInstall("mysql-8.4-lts"), aptPackages: []string{"mysql-server-8.4", "mysql-client"}, serviceName: "mysql"},
	"mariadb/10.11": {preInstall: mariadbPreInstall("10.11"), aptPackages: []string{"mariadb-server", "mariadb-client"}, serviceName: "mariadb"},
	"mariadb/11.4":  {preInstall: mariadbPreInstall("11.4"), aptPackages: []string{"mariadb-server", "mariadb-client"}, serviceName: "mariadb"},
	"picodata/25.3": {
		preInstall: []string{
			`curl -fsSL https://download.picodata.io/tarantool-picodata/picodata.gpg.key | gpg --no-default-keyring --keyring gnupg-ring:/etc/apt/trusted.gpg.d/picodata.gpg --import && chmod 644 /etc/apt/trusted.gpg.d/picodata.gpg`,
			`echo "deb https://download.picodata.io/tarantool-picodata/ubuntu/ $(lsb_release -cs) main" > /etc/apt/sources.list.d/picodata.list`,
			"apt-get update",
		},
		aptPackages: []string{"picodata"}, serviceName: "picodata",
	},
	"cockroach/24.2": {
		preInstall: []string{
			`curl -fsSL https://binaries.cockroachdb.com/cockroach-v24.2.0.linux-amd64.tgz -o /tmp/cockroach.tgz`,
			`tar -xzf /tmp/cockroach.tgz -C /tmp`,
			`install /tmp/cockroach-v24.2.0.linux-amd64/cockroach /usr/local/bin/cockroach`,
		},
		startScript: `cockroach start-single-node --insecure --background --listen-addr=0.0.0.0:26257 --sql-addr=0.0.0.0:5432`,
	},
	"ydb/25.3": {
		preInstall: []string{
			`curl -fsSL https://binaries.ydb.tech/release/25.3.0/ydbd-25.3.0-linux-amd64.tar.gz -o /tmp/ydbd.tgz`,
			`mkdir -p /opt/ydb && tar -xzf /tmp/ydbd.tgz -C /opt/ydb --strip-components=1`,
			`ln -sf /opt/ydb/bin/ydbd /usr/local/bin/ydbd`,
		},
		// Start + cluster/database bootstrap are render COMMAND items (config.yaml +
		// ydbd storage start + blobstorage/database init), so no service/startScript here.
	},
}

// defaultVersion picks a version when the preset omits one.
var defaultVersion = map[string]string{
	"postgres": "16", "mysql": "8.4", "mariadb": "11.4", "picodata": "25.3",
	"cockroach": "24.2", "ydb": "25.3",
}

// ubuntuUniversePreInstall refreshes the package lists and enables universe (a
// minimal base image ships neither) for archive packages like mysql-server.
var ubuntuUniversePreInstall = []string{
	"apt-get update",
	"apt-get install -y software-properties-common",
	"add-apt-repository -y universe",
	"apt-get update",
}

var pgPreInstall = []string{
	// Prereqs: a minimal base image lacks wget/gnupg/lsb-release (a cloud VM has
	// them, a fresh container does not) — install before adding the pgdg repo.
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

func mariadbPreInstall(version string) []string {
	return []string{
		`apt-get update`,
		`apt-get install -y curl ca-certificates`,
		`curl -fsSL https://r.mariadb.com/downloads/mariadb_repo_setup -o /tmp/mariadb_repo_setup`,
		`bash /tmp/mariadb_repo_setup --mariadb-server-version=` + version,
		`apt-get update`,
	}
}

// Per-kind recipes for the non-DATABASE components of an emergent HA/cluster
// topology. Configs are data (component.config); these carry only install+service.
var (
	// etcd/haproxy/node-exporter live in the distro repos; refresh the package
	// lists first (a minimal base image ships none) + enable universe for etcd.
	etcdRecipe = recipe{
		preInstall:  []string{"apt-get update", "apt-get install -y software-properties-common", "add-apt-repository -y universe", "apt-get update"},
		aptPackages: []string{"etcd-server", "etcd-client"}, serviceName: "etcd",
	}
	haproxyRecipe  = recipe{preInstall: []string{"apt-get update"}, aptPackages: []string{"haproxy"}, serviceName: "haproxy"}
	proxysqlRecipe = recipe{
		preInstall: []string{
			`apt-get install -y curl ca-certificates gnupg`,
			`install -d /etc/apt/keyrings`,
			`curl -fsSL https://repo.proxysql.com/ProxySQL/proxysql-2.x/repo_pub_key | gpg --dearmor -o /etc/apt/keyrings/proxysql.gpg`,
			`bash -c 'echo "deb [signed-by=/etc/apt/keyrings/proxysql.gpg] https://repo.proxysql.com/ProxySQL/proxysql-2.x/$(lsb_release -cs)/ ./" > /etc/apt/sources.list.d/proxysql.list'`,
			`apt-get update`,
		},
		aptPackages: []string{"proxysql"}, serviceName: "proxysql",
	}
	monitorRecipe = recipe{preInstall: []string{"apt-get update"}, aptPackages: []string{"prometheus-node-exporter"}, serviceName: "prometheus-node-exporter"}
)

// recipeForComponent resolves the install recipe for one topology component. The
// DATABASE recipe varies with the engine's replication mode (e.g. Patroni manages
// postgres, so the started service is patroni, not postgresql).
func recipeForComponent(c *domain.Topology_Component, db *domain.Database) recipe {
	switch c.GetKind() {
	case domain.Topology_Component_KIND_DATABASE:
		return databaseRecipe(db)
	case domain.Topology_Component_KIND_COORDINATOR:
		return etcdRecipe
	case domain.Topology_Component_KIND_PROXY:
		return proxyRecipe(db)
	case domain.Topology_Component_KIND_MONITOR:
		return monitorRecipe
	default:
		// STROPPY (workload binary preinstalled in the agent image), AGENT, ADDON:
		// no install steps; STROPPY contributes the run_stroppy command instead.
		return recipe{}
	}
}

// databaseRecipe is the engine recipe, adjusted for the replication mode.
func databaseRecipe(db *domain.Database) recipe {
	r := recipeFor(db)
	if db.GetKind() == domain.Database_KIND_POSTGRES &&
		db.GetOptions().GetPostgres().GetReplication().GetMode() == domain.Database_Options_Postgres_Replication_MODE_PATRONI {
		// Patroni supervises postgres + talks to etcd. Install patroni + the etcd
		// client; the start script drops the auto-created default cluster (patroni
		// initdb's its own), stops the package's postgresql service, and runs patroni
		// as the postgres user with OUR config (the debian unit hard-codes a
		// different config path) via a transient systemd unit.
		ver := pgVersion(db.GetVersion())
		r.aptPackages = append(append([]string{}, r.aptPackages...), "patroni", "python3-etcd")
		r.serviceName = ""
		r.startScript = strings.Join([]string{
			"pg_dropcluster --stop " + ver + " main >/dev/null 2>&1 || true",
			"systemctl disable --now postgresql >/dev/null 2>&1 || true",
			"install -d -o postgres -g postgres /var/lib/postgresql/" + ver + "/main",
			"chown -R postgres:postgres /etc/patroni",
			"systemd-run --unit=stroppy-patroni --uid=postgres --gid=postgres " +
				"--setenv=PATH=/usr/lib/postgresql/" + ver + "/bin:/usr/local/bin:/usr/bin:/bin " +
				"--collect /usr/bin/patroni /etc/patroni/patroni.yml",
		}, " && ")
	}
	return r
}

// pgVersion defaults the postgres major version.
func pgVersion(v string) string {
	if v == "" {
		return "16"
	}
	return v
}

// proxyRecipe picks the proxy engine: haproxy for postgres, proxysql for mysql/mariadb.
func proxyRecipe(db *domain.Database) recipe {
	switch db.GetKind() {
	case domain.Database_KIND_MYSQL, domain.Database_KIND_MARIADB:
		return proxysqlRecipe
	default:
		return haproxyRecipe
	}
}

// recipeFor resolves the install recipe for a database (by kind+version, falling
// back to the kind default). An unknown engine yields an empty recipe (no install
// steps) — TODO(planner): ydb/cockroach binary install.
func recipeFor(db *domain.Database) recipe {
	kind := kindString(db.GetKind())
	version := db.GetVersion()
	if version == "" {
		version = defaultVersion[kind]
	}
	if r, ok := recipes[kind+"/"+version]; ok {
		return r
	}
	return recipes[kind+"/"+defaultVersion[kind]]
}

func kindString(k domain.Database_Kind) string {
	switch k {
	case domain.Database_KIND_POSTGRES:
		return "postgres"
	case domain.Database_KIND_MYSQL:
		return "mysql"
	case domain.Database_KIND_MARIADB:
		return "mariadb"
	case domain.Database_KIND_PICODATA:
		return "picodata"
	case domain.Database_KIND_YDB:
		return "ydb"
	case domain.Database_KIND_COCKROACH:
		return "cockroach"
	default:
		return ""
	}
}

// topologyView extracts what the planner needs from a topology: the
// component->machine map (for binding resolution) and the primary DATABASE
// component (+ its machine memory for config sizing).
type topologyView struct {
	componentToMachine map[string]string
	dbComponentID      string
	dbMemoryMB         int
}

func viewTopology(topo *domain.Topology) topologyView {
	v := topologyView{componentToMachine: map[string]string{}}
	for _, m := range topo.GetMachines() {
		for _, c := range m.GetComponents() {
			v.componentToMachine[c.GetId()] = m.GetId()
			if c.GetKind() == domain.Topology_Component_KIND_DATABASE && v.dbComponentID == "" {
				v.dbComponentID = c.GetId()
				v.dbMemoryMB = int(m.GetMemoryGb()) * 1024
			}
		}
	}
	return v
}

// componentConfig returns a component's on-host config: the pre-rendered
// Component.config when present (wizard, preview==execution), else rendered on the
// fly by the shared render layer (same artifact either way, B3).
func componentConfig(c *domain.Topology_Component, db *domain.Database, topo *domain.Topology, memoryMB int) (*renderpb.Config, error) {
	if cfg := c.GetConfig(); len(cfg.GetItems()) > 0 {
		return cfg, nil
	}
	return render.RenderComponent(c, db, topo, memoryMB)
}
