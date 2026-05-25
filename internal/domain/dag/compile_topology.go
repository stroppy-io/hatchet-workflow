package dag

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

// CompileTopology lowers a Database (the user's structural intent) plus a Sizing
// (the user's hardware choice) into a Topology — the virtual IR that sits between
// intent and rendering. The Database dictates STRUCTURE: which component roles exist,
// the replication / coordination variant, and the connection graph between them. The
// Sizing dictates HARDWARE only: cores / memory / disk per role (the Database proto
// "intentionally avoids machine sizing"). The renderer and recipe layers must consume
// ONLY the returned Topology, never reach back into Database.Options.
//
// Pipeline: Database + Sizing → [CompileTopology] → Topology → [render] → configs.
// (One layer up, MaterializeDeploymentIntent lowers Topology + provider → Deployment.)
//
// NOTE (Phase 1): a few node counts that are STRUCTURE but not yet present in
// Database.Options — proxy count, picodata instance count — are carried on Sizing as
// a transition bridge (see Sizing.Proxies / Sizing.Instances). Phase 2 moves them into
// Database.Options and the compiler reads them from there.
func CompileTopology(db *domain.Database, size *Sizing) (*domain.Topology, error) {
	if db == nil {
		return nil, fmt.Errorf("compile topology: database is required")
	}
	if size == nil {
		size = &Sizing{}
	}
	var topo *domain.Topology
	switch db.GetKind() {
	case domain.Database_KIND_POSTGRES:
		topo = compilePostgres(db, size)
	case domain.Database_KIND_MYSQL, domain.Database_KIND_MARIADB:
		topo = compileMysql(db, size)
	case domain.Database_KIND_PICODATA:
		topo = compilePicodata(db, size)
	case domain.Database_KIND_YDB:
		topo = compileYDB(db, size)
	case domain.Database_KIND_COCKROACH:
		topo = compileCockroach(db, size)
	default:
		return nil, fmt.Errorf("compile topology: unsupported kind %v", db.GetKind())
	}
	// Bake each component's on-host Config into the component itself, rendered from the
	// Database intent + the component's known role (no connection inference). The baked
	// Topology is self-contained: render/runtime read Component.Config directly, and the
	// user sees + edits these configs. memory sizes the percent-based engine defaults.
	if err := bakeComponentConfigs(db, topo); err != nil {
		return nil, err
	}
	return topo, nil
}

// bakeComponentConfigs renders every component's Config in place. The renderer reads
// only the component (kind + role) and topology peers (by component kind), never
// Database.Options nor connections.
func bakeComponentConfigs(db *domain.Database, topo *domain.Topology) error {
	for _, m := range topo.GetMachines() {
		memoryMB := int(m.GetMemoryGb()) * 1024
		for _, c := range m.GetComponents() {
			cfg, err := render.RenderComponent(c, db, topo, memoryMB)
			if err != nil {
				return fmt.Errorf("bake config for component %q: %w", c.GetId(), err)
			}
			c.Config = cfg
		}
	}
	return nil
}

// Flavor is the hardware spec for one machine: cores + memory + boot disk, plus any
// secondary data disks (e.g. YDB pdisks). All of this is the user's hardware choice,
// never derived from Database.
type Flavor struct {
	Cores       uint32
	MemGb       uint64
	DiskGb      uint64
	DataDisksGb []uint64
}

// Sizing is the per-role hardware overlay applied onto the structure the Database
// implies. Only the roles a given preset uses need be set.
type Sizing struct {
	Database    Flavor // primary / replica / instance / single node
	Coordinator Flavor // etcd
	Proxy       Flavor // haproxy / proxysql
	Storage     Flavor // YDB storage tier (carries DataDisksGb = pdisks)
	Compute     Flavor // YDB compute (dynamic) tier

	// Phase-1 structural counts not yet in Database.Options (move to proto in Phase 2):
	Proxies   uint32 // proxy node count (pg / mysql / picodata)
	Instances uint32 // picodata instance count
}

// ── lowering: structure from Database.Options, hardware from Sizing ───────────

func compilePostgres(db *domain.Database, size *Sizing) *domain.Topology {
	repl := db.GetOptions().GetPostgres().GetReplication()
	// SINGLE: one universal node, no replication structure.
	if repl.GetMode() != domain.Database_Options_Postgres_Replication_MODE_PATRONI {
		return single(size.Database, "m-db1", "pg")
	}
	// PATRONI: etcd coordinator + (replicas+1) DB nodes + proxies. db1 is the primary;
	// every node coordinates with etcd; every replica streams from db1; proxies front db1.
	replicas := int(repl.GetReplicas())
	machines := []*domain.Topology_Machine{mach(size.Coordinator, "m-etcd", comp("etcd1", domain.Topology_Component_KIND_COORDINATOR))}
	var conns []*domain.Topology_Connection
	for i := 0; i <= replicas; i++ {
		id := dbID(i)
		machines = append(machines, mach(size.Database, "m-"+id, compRole(id, domain.Topology_Component_KIND_DATABASE, render.RolePatroni)))
		conns = append(conns, topoEdge(id, "etcd1", domain.Topology_Connection_KIND_COORDINATION))
		if i > 0 {
			conns = append(conns, topoEdge("db1", id, domain.Topology_Connection_KIND_REPLICATION))
		}
	}
	machines, conns = appendProxies(machines, conns, size, "db1")
	return &domain.Topology{Machines: machines, Connections: conns}
}

func compileMysql(db *domain.Database, size *Sizing) *domain.Topology {
	repl := db.GetOptions().GetMysql().GetReplication()
	if repl.GetMode() == domain.Database_Options_Mysql_Replication_MODE_SINGLE ||
		repl.GetMode() == domain.Database_Options_Mysql_Replication_Mode(0) {
		return single(size.Database, "m-db1", "db1")
	}
	// MULTI: db1 primary + N replicas streaming from it + proxies fronting db1.
	replicas := int(repl.GetReplicas())
	machines := []*domain.Topology_Machine{mach(size.Database, "m-db1", compRole("db1", domain.Topology_Component_KIND_DATABASE, render.RolePrimary))}
	var conns []*domain.Topology_Connection
	for i := 0; i < replicas; i++ {
		id := fmt.Sprintf("db%d", 2+i)
		machines = append(machines, mach(size.Database, "m-"+id, compRole(id, domain.Topology_Component_KIND_DATABASE, render.RoleReplica)))
		conns = append(conns, topoEdge("db1", id, domain.Topology_Connection_KIND_REPLICATION))
	}
	machines, conns = appendProxies(machines, conns, size, "db1")
	return &domain.Topology{Machines: machines, Connections: conns}
}

func compilePicodata(db *domain.Database, size *Sizing) *domain.Topology {
	instances := int(size.Instances)
	if instances <= 1 {
		return single(size.Database, "m-pd1", "pd1")
	}
	// CLUSTER: pd1 + raft peers coordinating with pd1 + proxies fronting pd1.
	machines := make([]*domain.Topology_Machine, 0, instances)
	var conns []*domain.Topology_Connection
	for i := 0; i < instances; i++ {
		id := fmt.Sprintf("pd%d", 1+i)
		machines = append(machines, mach(size.Database, "m-"+id, comp(id, domain.Topology_Component_KIND_DATABASE)))
		if i > 0 {
			conns = append(conns, topoEdge(id, "pd1", domain.Topology_Connection_KIND_COORDINATION))
		}
	}
	machines, conns = appendProxies(machines, conns, size, "pd1")
	return &domain.Topology{Machines: machines, Connections: conns}
}

func compileYDB(db *domain.Database, size *Sizing) *domain.Topology {
	sh := db.GetOptions().GetYdb().GetSelfHosted()
	storageNodes := int(sh.GetStorageNodes())
	computeNodes := int(sh.GetDatabaseNodes())
	// SINGLE universal node: no separate compute tier.
	if computeNodes == 0 {
		return single(size.Database, "m-ydb1", "ydb1")
	}
	// Compute/storage-separated: a storage tier (StorageNodes, each with pdisks) and a
	// compute tier (DatabaseNodes), all coordinating with the first storage node.
	machines := make([]*domain.Topology_Machine, 0, storageNodes+computeNodes)
	var conns []*domain.Topology_Connection
	for i := 1; i <= storageNodes; i++ {
		id := fmt.Sprintf("ydbs%d", i)
		machines = append(machines, mach(size.Storage, "m-"+id, comp(id, domain.Topology_Component_KIND_DATABASE)))
		if i > 1 {
			conns = append(conns, topoEdge(id, "ydbs1", domain.Topology_Connection_KIND_COORDINATION))
		}
	}
	for i := 1; i <= computeNodes; i++ {
		id := fmt.Sprintf("ydbc%d", i)
		machines = append(machines, mach(size.Compute, "m-"+id, comp(id, domain.Topology_Component_KIND_DATABASE)))
		conns = append(conns, topoEdge(id, "ydbs1", domain.Topology_Connection_KIND_COORDINATION))
	}
	return &domain.Topology{Machines: machines, Connections: conns}
}

func compileCockroach(db *domain.Database, size *Sizing) *domain.Topology {
	nodes := int(db.GetOptions().GetCockroach().GetNodes())
	if nodes <= 1 {
		return single(size.Database, "m-crdb1", "crdb1")
	}
	machines := make([]*domain.Topology_Machine, 0, nodes)
	var conns []*domain.Topology_Connection
	for i := 0; i < nodes; i++ {
		id := fmt.Sprintf("crdb%d", 1+i)
		role := render.RoleReplica
		if i == 0 {
			role = render.RolePrimary // crdb1 is the cluster seed (runs init)
		}
		machines = append(machines, mach(size.Database, "m-"+id, compRole(id, domain.Topology_Component_KIND_DATABASE, role)))
		if i > 0 {
			conns = append(conns, topoEdge(id, "crdb1", domain.Topology_Connection_KIND_COORDINATION))
		}
	}
	return &domain.Topology{Machines: machines, Connections: conns}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// single is the one-machine topology shared by every engine's single-node preset: a
// lone DATABASE component plus a SUPPORT self-edge (satisfies the topology's
// min-1-connection rule; inert to the dag builder).
func single(f Flavor, machineID, componentID string) *domain.Topology {
	return &domain.Topology{
		Machines:    []*domain.Topology_Machine{mach(f, machineID, comp(componentID, domain.Topology_Component_KIND_DATABASE))},
		Connections: []*domain.Topology_Connection{selfEdge(componentID)},
	}
}

// appendProxies adds size.Proxies PROXY nodes (px1..pxN) each fronting target.
func appendProxies(machines []*domain.Topology_Machine, conns []*domain.Topology_Connection, size *Sizing, target string) ([]*domain.Topology_Machine, []*domain.Topology_Connection) {
	for i := 0; i < int(size.Proxies); i++ {
		id := fmt.Sprintf("px%d", 1+i)
		machines = append(machines, mach(size.Proxy, "m-"+id, comp(id, domain.Topology_Component_KIND_PROXY)))
		conns = append(conns, topoEdge(id, target, domain.Topology_Connection_KIND_PROXY))
	}
	return machines, conns
}

func dbID(i int) string { return fmt.Sprintf("db%d", 1+i) }

func cfgID(id string) *renderpb.Config { return &renderpb.Config{Id: id} }

func comp(id string, kind domain.Topology_Component_Kind) *domain.Topology_Component {
	return &domain.Topology_Component{Id: id, Kind: kind, Config: cfgID(id)}
}

// compRole builds a component carrying an explicit cluster-role label — the role is a
// first-class, user-visible property of the IR (render reads it; never re-derived from
// options or connections).
func compRole(id string, kind domain.Topology_Component_Kind, role string) *domain.Topology_Component {
	c := comp(id, kind)
	c.Tags = &common.Tags{Labels: map[string]string{render.RoleLabelKey: role}}
	return c
}

func mach(f Flavor, id string, comps ...*domain.Topology_Component) *domain.Topology_Machine {
	return &domain.Topology_Machine{
		Id: id, Cores: f.Cores, MemoryGb: f.MemGb, DiskGb: f.DiskGb,
		DataDisksGb: f.DataDisksGb, Components: comps,
	}
}

func topoEdge(from, to string, kind domain.Topology_Connection_Kind) *domain.Topology_Connection {
	return &domain.Topology_Connection{From: from, To: to, Kind: kind}
}

func selfEdge(id string) *domain.Topology_Connection {
	return &domain.Topology_Connection{From: id, To: id, Kind: domain.Topology_Connection_KIND_SUPPORT}
}
