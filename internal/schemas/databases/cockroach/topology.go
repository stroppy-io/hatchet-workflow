package cockroach

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// topologyFields are the root discriminators of the logical topology. There is
// NO machine/provider/instance-spec here — only the logical shape, the engine
// version and the node count. CockroachDB nodes are homogeneous and symmetric
// (no master/replica), so a "cluster" is fully described by node_count plus the
// replication factor in the cluster section. The cluster schema derives the
// backing machines from these.
func topologyFields() []schemapb.FieldDef {
	return []schemapb.FieldDef{
		utils.StrEnum(FieldTopology, TopologyValues...).
			Default(TopologySingle).Required().
			Title("Topology").
			Desc("Cluster shape. Root discriminator. single: one node (no --join, " +
				"replication factor pinned to 1). cluster: N homogeneous gossip-joined " +
				"nodes with a replicated keyspace (no master/replica roles)."),

		utils.StrEnum(FieldVersion, CrdbMajorValues...).Default(CrdbMajor242).Required().
			Title("CockroachDB version").
			Desc("Major release line (e.g. 24.2); the exact patch tarball is resolved downstream."),

		schemapb.Int32(FieldNodeCount).Gte(1).Lte(64).Default(1).
			Title("Node count").
			Desc("Number of homogeneous nodes (logical). Should be 1 for single, and " +
				">= replication_factor for cluster. Backing machines are allocated by the " +
				"cluster schema."),
	}
}
