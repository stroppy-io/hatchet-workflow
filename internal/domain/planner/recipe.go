package planner

import (
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/compat"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

// stroppyDBHostToken is the late-binding hole in the run_stroppy command,
// resolved to the database component's private ip at the plan->execute seam.
const stroppyDBHostToken = "__STROPPY_DB_HOST__"

// recipe is the per-component install plan. The DATA (the engine compat matrix +
// install recipes) lives in package compat (H29: backend data); this package holds
// only the SELECTION LOGIC over it (which recipe a component gets + shape-specific
// adjustments like patroni).
type recipe = compat.Recipe

// recipeForComponent resolves the install recipe for one topology component. The
// DATABASE recipe varies with the engine's replication mode (e.g. Patroni manages
// postgres, so the started service is patroni, not postgresql).
func recipeForComponent(c *domain.Topology_Component, db *domain.Database) recipe {
	switch c.GetKind() {
	case domain.Topology_Component_KIND_DATABASE:
		return databaseRecipe(db)
	case domain.Topology_Component_KIND_COORDINATOR:
		return compat.Etcd
	case domain.Topology_Component_KIND_PROXY:
		return proxyRecipe(db)
	case domain.Topology_Component_KIND_MONITOR:
		return compat.Monitor
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
		r.AptPackages = append(append([]string{}, r.AptPackages...), "patroni", "python3-etcd")
		r.ServiceName = ""
		r.StartScript = strings.Join([]string{
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
		return compat.ProxySQL
	default:
		return compat.HAProxy
	}
}

// recipeFor resolves the install recipe for a database (by kind+version) from the
// compat matrix. An unknown engine yields an empty recipe (no install steps).
func recipeFor(db *domain.Database) recipe {
	r, _ := compat.EngineRecipe(kindString(db.GetKind()), db.GetVersion())
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
