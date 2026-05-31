package recipe

import "github.com/stroppy-io/stroppy-cloud/internal/schemas/expand"

// Node is one deployed machine: its topology instance id, runtime IP and role.
type Node struct {
	ID   string
	IP   string
	Role expand.Role
}

// Cluster is the full deployed-topology view handed to every step builder so it
// can render cross-node config (etcd initial-cluster, patroni etcd hosts, haproxy
// backends, group-replication seeds, cockroach --join, ...). All IPs are known at
// build time (post-deploy), so configs are rendered concretely — no runtime
// placeholders for cluster addressing.
type Cluster struct {
	// Self is the node these steps are being built for.
	Self Node
	// All is every deployed node (any role), in topology order.
	All []Node
	// Role groups nodes by role (Role[RoleCoordinator] = all etcd nodes, ...).
	Role map[expand.Role][]Node
}

// IPs returns the IPs of all nodes with the given role, in order.
func (c Cluster) IPs(role expand.Role) []string {
	nodes := c.Role[role]
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.IP)
	}
	return out
}

// Nodes returns all nodes with the given role.
func (c Cluster) Nodes(role expand.Role) []Node { return c.Role[role] }

// First returns the first node with the given role (ok=false when none).
func (c Cluster) First(role expand.Role) (Node, bool) {
	if nodes := c.Role[role]; len(nodes) > 0 {
		return nodes[0], true
	}
	return Node{}, false
}

// Has reports whether any node plays the given role.
func (c Cluster) Has(role expand.Role) bool { return len(c.Role[role]) > 0 }

// Index returns Self's position among nodes of its own role (0-based). Used for
// per-node identity (etcd<i>, pg<i>, server-id i+1, instance-<i>).
func (c Cluster) Index() int {
	for i, n := range c.Role[c.Self.Role] {
		if n.ID == c.Self.ID {
			return i
		}
	}
	return 0
}

// NewCluster builds a Cluster for self from the full node list.
func NewCluster(self Node, all []Node) Cluster {
	byRole := make(map[expand.Role][]Node)
	for _, n := range all {
		byRole[n.Role] = append(byRole[n.Role], n)
	}
	return Cluster{Self: self, All: all, Role: byRole}
}
