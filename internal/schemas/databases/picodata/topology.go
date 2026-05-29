package picodata

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// topologyFields are the root discriminators of the logical topology. There is
// NO machine/provider/placement here — only the logical cluster shape: how many
// instances, the cluster-wide default replication factor, and the cluster name.
// The cluster schema derives instances/machines from these.
func topologyFields() []schemapb.FieldDef {
	return []schemapb.FieldDef{
		utils.StrEnum(FieldTopology, TopologyValues...).
			Default(TopologySingle).Required().
			Title("Topology").
			Desc("Cluster shape. Root discriminator. single: one instance (one " +
				"replicaset, no data spread). cluster: multiple instances grouped " +
				"into replicasets/tiers with raft + vshard sharding."),

		schemapb.Str(FieldClusterName).Default("stroppy-cluster").Required().MinLen(1).
			Title("Cluster name").
			Desc("cluster.name in picodata.yaml. All instances of a cluster must share it."),

		schemapb.Int32(FieldInstanceCount).Gte(1).Lte(256).Default(1).Required().
			Title("Instance count").
			Desc("Total number of picodata instances (logical). For topology=single this " +
				"is 1. Instances are grouped into replicasets of size replication_factor; " +
				"backing machines are allocated by the cluster schema."),

		schemapb.Int32(FieldReplicationFactor).Gte(1).Lte(32).Default(1).Required().
			Title("Default replication factor").
			Desc("cluster.tier.default.replication_factor: instances per replicaset. " +
				"1 = no replication. 3 is the common production choice (quorum on raft). " +
				"Per-tier overrides live in the tiers list below."),
	}
}

// shardingSection models vshard sharding as a first-class concept (the old code
// declared Shards but never wired it). Sharding splits data into buckets spread
// across replicasets; it is meaningful only for a multi-instance cluster.
func shardingSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldSharding,
		schemapb.Bool(FieldEnabled).Default(true).
			Title("Enable sharding").
			Desc("Spread sharded tables across replicasets via vshard. "+
				"Disable for a single-replicaset cluster."),

		schemapb.Int32(FieldBucketCount).Gte(1).Lte(1000000).Default(3000).
			When(utils.IsTrue(rp(pfx, FieldSharding, FieldEnabled))).
			Title("Bucket count").
			Desc("Total vshard buckets the keyspace is split into (cluster-wide). "+
				"Rule of thumb: ~100-1000 buckets per replicaset; default 3000 suits "+
				"small clusters. Fixed for the cluster lifetime."),

		schemapb.Bool(FieldRebalancer).Default(true).
			When(utils.IsTrue(rp(pfx, FieldSharding, FieldEnabled))).
			Title("Automatic rebalancer").
			Desc("Let vshard move buckets to keep replicasets balanced."),
	).When(utils.Eq(rp(pfx, FieldTopology), TopologyCluster)).
		Title("Sharding")
}

// tiersSection models tiers as first-class (the old code declared Tiers but
// always emitted a single default tier). A tier is a named group of instances
// with its own replication_factor, voting capability and (logical) instance
// count — e.g. a "storage" tier and a "compute"/"router" tier. Active only for a
// multi-instance cluster; a single instance is implicitly the sole default tier.
func tiersSection(pfx string) schemapb.FieldDef {
	return schemapb.List(FieldTiers,
		schemapb.Object(FieldTier,
			schemapb.Str(FieldName).Required().MinLen(1).
				Title("Tier name").
				Desc("cluster.tier.<name>. Must be unique. 'default' is the implicit base tier."),

			schemapb.Int32(FieldReplicationFactor).Gte(1).Lte(32).Default(1).
				Title("Replication factor").
				Desc("Instances per replicaset within this tier. Overrides the cluster default."),

			schemapb.Bool(FieldCanVote).Default(true).
				Title("Can vote").
				Desc("Whether instances of this tier may participate in raft leader elections. "+
					"Set false for non-voting/read-scaling tiers (keep voters odd for quorum)."),

			schemapb.Int32(FieldCount).Gte(0).Lte(256).Default(0).
				Title("Instance count").
				Desc("Logical number of instances assigned to this tier. 0 = derive from the "+
					"cluster-level instance_count. Backing machines allocated by the cluster schema."),

			schemapb.Bool(FieldIsSharded).Default(true).
				Title("Sharded").
				Desc("Whether this tier participates in vshard bucket distribution. "+
					"Set false for a small coordinator/router-only tier."),
		).Title("Tier"),
	).When(utils.Eq(rp(pfx, FieldTopology), TopologyCluster)).
		Title("Tiers").
		Desc("Named instance groups with per-tier replication. Empty = single implicit " +
			"'default' tier sized from instance_count / replication_factor.")
}
