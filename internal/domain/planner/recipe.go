package planner

import (
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
	"postgres/16":   {preInstall: pgPreInstall, aptPackages: []string{"postgresql-16", "postgresql-client-16"}, serviceName: "postgresql"},
	"postgres/17":   {preInstall: pgPreInstall, aptPackages: []string{"postgresql-17", "postgresql-client-17"}, serviceName: "postgresql"},
	"mysql/8.0":     {preInstall: mysqlPreInstall("mysql-8.0"), aptPackages: []string{"mysql-server-8.0", "mysql-client"}, serviceName: "mysql"},
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
		},
		// TODO(planner): single-node YDB also needs a static storage config + blobstorage
		// bootstrap (ydbd admin blobstorage init) — not a single start command. Recast
		// the old setup_ydb static/dynamic node sequence.
		startScript: `echo "TODO: ydb single-node bootstrap not implemented"`,
	},
}

// defaultVersion picks a version when the preset omits one.
var defaultVersion = map[string]string{
	"postgres": "16", "mysql": "8.4", "mariadb": "11.4", "picodata": "25.3",
	"cockroach": "24.2", "ydb": "25.3",
}

var pgPreInstall = []string{
	`sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'`,
	"wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -",
	"apt-get update",
}

func mysqlPreInstall(component string) []string {
	return []string{
		`apt-get install -y curl gnupg lsb-release ca-certificates`,
		`install -d /etc/apt/keyrings`,
		`curl -fsSL https://repo.mysql.com/RPM-GPG-KEY-mysql-2023 | gpg --dearmor -o /etc/apt/keyrings/mysql.gpg`,
		`bash -c 'echo "deb [signed-by=/etc/apt/keyrings/mysql.gpg] http://repo.mysql.com/apt/ubuntu/ $(lsb_release -cs) ` + component + `" > /etc/apt/sources.list.d/mysql.list'`,
		`apt-get update`,
	}
}

func mariadbPreInstall(version string) []string {
	return []string{
		`curl -fsSL https://r.mariadb.com/downloads/mariadb_repo_setup -o /tmp/mariadb_repo_setup`,
		`bash /tmp/mariadb_repo_setup --mariadb-server-version=` + version,
		`apt-get update`,
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

// databaseConfig returns the rendered DB config: the intent's pre-rendered
// Database.config when present (wizard, preview==execution), else rendered on the
// fly. Returns an empty Config (no config writes) for engines the renderer does
// not support yet.
func databaseConfig(db *domain.Database, memoryMB int) *renderpb.Config {
	if cfg := db.GetConfig(); len(cfg.GetItems()) > 0 {
		return cfg
	}
	cfg, err := render.RenderDatabase(db, memoryMB)
	if err != nil {
		return &renderpb.Config{} // TODO(planner): non-postgres engines
	}
	return cfg
}
