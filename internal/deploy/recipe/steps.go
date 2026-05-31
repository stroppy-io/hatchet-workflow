package recipe

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/expand"
)

// StepKind is how the workflow dispatches a Step to an agent activity.
type StepKind string

const (
	StepWriteFile StepKind = "write_file" // WriteFileActivity
	StepFetchFile StepKind = "fetch_file" // FetchFileActivity (download by URI)
	StepCmd       StepKind = "cmd"        // CallCmdActivity
)

// Step is ONE discrete, labeled deploy action. The workflow runs each Step as a
// single agent activity AND surfaces it as its own pipeline node in the overview
// (Name → node name), so each install/config/run action is observable, retryable
// and log-sliceable on its own. This replaces the old "one mega bash script per
// phase" shape.
//
// Fields are PLAIN (no proto) so a Step round-trips cleanly across the Temporal
// activity boundary (proto oneofs do not survive the default JSON converter); the
// workflow rebuilds the common.File / common.Cmd from these when it dispatches.
type Step struct {
	Name string
	Kind StepKind

	// write_file / fetch_file
	Path string
	Mode uint32
	// write_file
	Content string
	// fetch_file
	URI      string
	Checksum string
	// cmd (bash script)
	Script string
}

// stepRecipe is the granular side of a Recipe: given the node being built (Self)
// and the full deployed Cluster view, it returns that node's ordered deploy
// steps. Cross-node config is rendered concretely from the Cluster (all IPs known
// at build time).
type stepRecipe interface {
	Steps(self Node, cl Cluster, db, wl map[string]any, refs Refs) []Step
}

// Steps returns the ordered discrete steps for one node of the given db kind.
// Returns nil for an unregistered kind / a recipe without a granular impl.
func Steps(kind string, self Node, cl Cluster, db, wl map[string]any, refs Refs) []Step {
	r, ok := recipes[kind]
	if !ok {
		return nil
	}
	sr, ok := r.(stepRecipe)
	if !ok {
		return nil
	}
	return sr.Steps(self, cl, db, wl, refs)
}

// --- step constructors ---

func cmdStep(name, script string) Step {
	return Step{Name: name, Kind: StepCmd, Script: script}
}

func writeStep(name, path string, mode uint32, content string) Step {
	return Step{Name: name, Kind: StepWriteFile, Path: path, Mode: mode, Content: content}
}

func fetchStep(name, path string, mode uint32, uri, checksum string) Step {
	return Step{Name: name, Kind: StepFetchFile, Path: path, Mode: mode, URI: uri, Checksum: checksum}
}

// --- postgres granular steps ---

// Steps implements stepRecipe for postgres. Routing by role + cluster shape:
// with a DCS quorum (coordinators) present the DB nodes run Patroni HA; otherwise
// a single plain postgres node (the simple docker demo).
func (pgRecipe) Steps(self Node, cl Cluster, db, wl map[string]any, refs Refs) []Step {
	ha := cl.Has(expand.RoleCoordinator)
	switch self.Role {
	case expand.RoleCoordinator:
		return etcdSteps(self, cl)
	case expand.RoleDatabase, expand.RoleReplica:
		if ha {
			return pgPatroniSteps(self, cl, db)
		}
		return pgDatabaseSteps(db)
	case expand.RolePooler:
		return pgBouncerSteps()
	case expand.RoleProxy:
		return haproxyPostgresSteps(cl)
	case expand.RoleWorkload:
		return pgWorkloadSteps(self, cl, db, wl, refs)
	default:
		return nil
	}
}

func pgDatabaseSteps(db map[string]any) []Step {
	major := pgMajor(db)
	return []Step{
		writeStep("write postgresql.conf", "/etc/postgresql/postgresql.conf", 0o644, renderPostgresConf(db)),
		writeStep("write pg_hba.conf", "/etc/postgresql/pg_hba.conf", 0o640, renderPgHBA()),
		cmdStep("apt update", "set -e\nexport DEBIAN_FRONTEND=noninteractive\napt-get update"),
		cmdStep("install apt deps", "set -e\nexport DEBIAN_FRONTEND=noninteractive\napt-get install -y curl ca-certificates gnupg lsb-release"),
		cmdStep("add PGDG key", "set -e\ninstall -d /usr/share/postgresql-common/pgdg\ncurl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.gpg"),
		cmdStep("add PGDG repo", `set -e
echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.gpg] http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list
apt-get update`),
		cmdStep(fmt.Sprintf("install postgresql-%d", major),
			fmt.Sprintf("set -e\nexport DEBIAN_FRONTEND=noninteractive\napt-get install -y postgresql-%d postgresql-client-%d", major, major)),
		cmdStep("apply config", fmt.Sprintf(`set -euo pipefail
CONFDIR="/etc/postgresql/%d/main"
mkdir -p "$CONFDIR/conf.d"
sed 's/__LISTEN__/*/g' /etc/postgresql/postgresql.conf > "$CONFDIR/conf.d/stroppy.conf"
cp /etc/postgresql/pg_hba.conf "$CONFDIR/pg_hba.conf"
chown -R postgres:postgres "$CONFDIR/conf.d" "$CONFDIR/pg_hba.conf"`, major)),
		cmdStep("start cluster", fmt.Sprintf("set -e\npg_ctlcluster %d main start || pg_ctlcluster %d main restart", major, major)),
		cmdStep("wait ready", fmt.Sprintf(`set -e
for i in $(seq 1 30); do pg_isready -h 127.0.0.1 -p %d && exit 0; sleep 1; done
pg_isready -h 127.0.0.1 -p %d`, pgListenPort, pgListenPort)),
		cmdStep("seed role", fmt.Sprintf(`set -e
su - postgres -c "psql -tc \"SELECT 1 FROM pg_roles WHERE rolname='%s'\" | grep -q 1 || psql -c \"CREATE ROLE %s LOGIN SUPERUSER PASSWORD '%s'\""`,
			pgDemoUser, pgDemoUser, pgDemoPass)),
		cmdStep("seed database", fmt.Sprintf(`set -e
su - postgres -c "psql -tc \"SELECT 1 FROM pg_database WHERE datname='%s'\" | grep -q 1 || psql -c \"CREATE DATABASE %s OWNER %s\""`,
			pgDemoDB, pgDemoDB, pgDemoUser)),
	}
}

// pgWorkloadSteps fetches stroppy and runs a real ~1-minute tpcc load. The DSN
// points at HAProxy (write port 5000) when a proxy is deployed, else directly at
// the (first) database node on 5432.
func pgWorkloadSteps(self Node, cl Cluster, db, wl map[string]any, refs Refs) []Step {
	host, port := pgEndpoint(cl)
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", pgDemoUser, pgDemoPass, host, port, pgDemoDB)
	return stroppyRunSteps(driverPostgres, "tpcc/procs", dsn, refs)
}

// pgEndpoint resolves the workload's connect target: HAProxy write port when a
// proxy is present, else the first database node on the postgres port.
func pgEndpoint(cl Cluster) (string, int) {
	if proxy, ok := cl.First(expand.RoleProxy); ok {
		return proxy.IP, haproxyWritePort
	}
	if dbn, ok := cl.First(expand.RoleDatabase); ok {
		return dbn.IP, pgListenPort
	}
	return "127.0.0.1", pgListenPort
}
