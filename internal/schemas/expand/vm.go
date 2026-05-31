// Package expand turns a baked, provider-agnostic database config (the values of
// a database schema) into the LOGICAL set of machines (VMs) a deployment needs —
// one entry per role, count correct BY CONSTRUCTION. There is no cross-schema
// validation here: the role→VM derivation cannot produce an inconsistent count.
//
// Each VM carries an abstract, provider-agnostic shape (cores/mem/disk) and a
// role. The provider overlay (disk class, zone, platform) and any capacity
// sanity checks are applied later, when a provider is chosen; the provider's
// own option lists constrain what a VM may use, so no count/catalog rule web is
// needed. The final deployment topology = database VMs ++ workload runner VMs.
package expand

// Role is the logical purpose of a machine in a deployment.
type Role string

const (
	RoleDatabase    Role = "database"    // primary / sole DB node
	RoleReplica     Role = "replica"     // streaming / cluster replica
	RoleCoordinator Role = "coordinator" // DCS / consensus (etcd, ...)
	RoleProxy       Role = "proxy"       // LB / router (HAProxy, ProxySQL, ...)
	RolePooler      Role = "pooler"      // dedicated connection pooler
	RoleWorkload    Role = "workload"    // stroppy load runner
	RoleMonitor     Role = "monitor"     // monitoring node
)

// Tier is the deploy ordering of a role across machines: lower tiers come up
// first, so dependencies are satisfied (consensus before the DB it backs, the DB
// before the proxy in front of it, everything before the workload that drives
// it). Machines in the same tier are independent and may deploy concurrently.
func (r Role) Tier() int {
	switch r {
	case RoleCoordinator: // etcd / DCS — must be up before Patroni
		return 0
	case RoleDatabase, RoleReplica:
		return 1
	case RolePooler:
		return 2
	case RoleProxy:
		return 3
	case RoleWorkload:
		return 4
	case RoleMonitor:
		return 5
	default:
		return 6
	}
}

// Shape is the abstract, provider-agnostic machine capacity. This is the WHOLE
// machine — there is no provider-specific data on a machine. Provider override
// parameters (disk class, zone, platform) are a SEPARATE, server-emitted schema
// computed from (db config × provider) and edited by the user; at bake the
// abstract machine maps to topology.Instance.machine_info and the override
// values map to Instance.provider_parms.
type Shape struct {
	Cores    int `json:"cores"`
	MemoryGB int `json:"memory_gb"`
	DiskGB   int `json:"disk_gb"`
}

// VM is one logical, ABSTRACT machine in a deployment (role + capacity shape).
// No provider fields — provider overrides live in the emitted override schema.
type VM struct {
	Role  Role   `json:"role"`
	Name  string `json:"name"`
	Shape Shape  `json:"shape"`
}

// Default per-role shapes — production-ish starting points the wizard/user can
// tune. DB/replica nodes are the heavy tier; coordinators (etcd) are light but
// latency-sensitive; the stroppy runner needs cores to generate load.
var (
	ShapeDatabase    = Shape{Cores: 8, MemoryGB: 32, DiskGB: 200}
	ShapeReplica     = Shape{Cores: 8, MemoryGB: 32, DiskGB: 200}
	ShapeCoordinator = Shape{Cores: 2, MemoryGB: 4, DiskGB: 20}
	ShapeProxy       = Shape{Cores: 4, MemoryGB: 8, DiskGB: 40}
	ShapePooler      = Shape{Cores: 4, MemoryGB: 8, DiskGB: 20}
	ShapeWorkload    = Shape{Cores: 16, MemoryGB: 32, DiskGB: 100}
	ShapeMonitor     = Shape{Cores: 4, MemoryGB: 8, DiskGB: 100}
)
