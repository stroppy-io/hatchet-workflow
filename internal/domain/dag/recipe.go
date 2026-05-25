package dag

import (
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

// stroppyDBHostToken is the late-binding hole in the run_stroppy command,
// resolved to the database component's private ip at the plan->execute seam.
const stroppyDBHostToken = "__STROPPY_DB_HOST__"

// recipeForComponent resolves the install recipe for one topology component. The
// DATABASE recipe varies with the engine's replication mode (e.g. Patroni manages
// postgres, so the started service is patroni, not postgresql). The recipe DATA
// itself lives in recipes_data.go; this is only the SELECTION LOGIC over it.
func recipeForComponent(c *domain.Topology_Component, db *domain.Database, topo *domain.Topology) Recipe {
	switch c.GetKind() {
	case domain.Topology_Component_KIND_DATABASE:
		return databaseRecipe(c, db, topo)
	case domain.Topology_Component_KIND_COORDINATOR:
		return recipeEtcd
	case domain.Topology_Component_KIND_PROXY:
		return proxyRecipe(db)
	case domain.Topology_Component_KIND_MONITOR:
		return recipeMonitor
	default:
		// STROPPY (workload binary preinstalled in the agent image), AGENT, ADDON:
		// no install steps; STROPPY contributes the run_stroppy command instead.
		return Recipe{}
	}
}

// databaseRecipe is the engine recipe, adjusted for the cluster role of the component
// as read from the topology IR (never from Database.Options).
func databaseRecipe(c *domain.Topology_Component, db *domain.Database, topo *domain.Topology) Recipe {
	r := recipeFor(db)
	if render.IsPatroniManaged(c) {
		// Patroni supervises postgres + talks to etcd. Install patroni + the etcd
		// client; the start script drops the auto-created default cluster (patroni
		// initdb's its own), stops the package's postgresql service, and runs patroni
		// as the postgres user with OUR config (the debian unit hard-codes a
		// different config path) via a transient systemd unit.
		ver := pgVersion(db.GetVersion())
		r.AptPackages = append(append([]string{}, r.AptPackages...), "patroni", "python3-etcd")
		r.ServiceName = ""
		r.StartScript = strings.Join([]string{
			// Idempotent: an agent's 60s command lease expires under cluster load
			// (5 replicas basebackup the primary at once), so this step gets
			// re-delivered. A second run of `systemd-run --unit=stroppy-patroni`
			// would fail with "unit already exists" and the node would fail
			// permanently. If patroni is already up from a prior delivery, succeed.
			"if systemctl is-active --quiet stroppy-patroni; then exit 0; fi",
			"pg_dropcluster --stop " + ver + " main >/dev/null 2>&1 || true",
			"systemctl disable --now postgresql >/dev/null 2>&1 || true",
			// Mode 0700: postgres refuses to start on a data dir with group/other
			// access. The primary's initdb forces 0700, but a replica's
			// pg_basebackup writes into this pre-created dir and keeps its mode, so
			// it must be 0700 from the start or PG aborts with "invalid permissions".
			"install -d -m 0700 -o postgres -g postgres /var/lib/postgresql/" + ver + "/main",
			"chown -R postgres:postgres /etc/patroni",
			// Clear a dead/failed transient unit from a prior delivery so --unit is free.
			"systemctl reset-failed stroppy-patroni >/dev/null 2>&1 || true",
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
func proxyRecipe(db *domain.Database) Recipe {
	switch db.GetKind() {
	case domain.Database_KIND_MYSQL, domain.Database_KIND_MARIADB:
		return recipeProxySQL
	default:
		return recipeHAProxy
	}
}

// recipeFor resolves the install recipe for a database (by kind+version) from the
// compat matrix. An unknown engine yields an empty recipe (no install steps).
func recipeFor(db *domain.Database) Recipe {
	r, _ := EngineRecipe(kindString(db.GetKind()), db.GetVersion())
	return r
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
