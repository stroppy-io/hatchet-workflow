package ydb

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// topologyFields are the root discriminators of the logical YDB topology. There
// is NO machine/provider/disk-hardware here — only the logical shape and node
// counts. The cluster schema derives instances/machines/zones from these, and
// owns the combined-mode memory-halving + pdisk SIZE auto-calc at deploy time.
//
// YDB always has a static storage tier. `topology` only decides whether the
// dynamic database/compute nodes share the storage nodes (combined) or run on
// their own (split).
func topologyFields(pfx string) []schemapb.FieldDef {
	return []schemapb.FieldDef{
		utils.StrEnum(FieldTopology, TopologyValues...).
			Default(TopologyCombined).Required().
			Title("Topology").
			Desc("Cluster shape. Root discriminator. combined: ydbd-storage and " +
				"ydbd-database run co-located on the storage nodes (each daemon's memory " +
				"hard limit is halved downstream to avoid OOM). split: dynamic " +
				"database/compute nodes run on separate nodes (full memory each)."),

		schemapb.Int32(FieldStorageNodeCount).Gte(1).Lte(255).Default(1).Required().
			Title("Storage node count").
			Desc("Number of static storage nodes (logical). block-4-2 typically needs " +
				">=8 nodes in one DC; mirror-3-dc needs >=3 nodes across 3 DCs — those " +
				"cross-field minimums are enforced at the cluster layer. Backing machines " +
				"are allocated by the cluster schema."),

		schemapb.Int32(FieldDatabaseNodeCount).Gte(0).Lte(255).Default(0).
			When(utils.Eq(rp(pfx, FieldTopology), TopologySplit)).
			Title("Database / compute node count").
			Desc("Number of dynamic database (compute) nodes. Only meaningful for " +
				"split topology; combined co-locates them on storage nodes."),

		schemapb.Str(FieldDatabasePath).Default("/Root/testdb").Required().
			Title("Database path").
			Desc("Tenant database path created via `ydbd admin database <path> create` " +
				"and used as the connection database (e.g. /Root/testdb)."),

		schemapb.Str(FieldTenant).Default("/Root/testdb").
			Title("Tenant").
			Desc("Tenant passed to dynamic nodes via --tenant. Usually equals database_path."),
	}
}
