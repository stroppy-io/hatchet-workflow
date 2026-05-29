package cockroach

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

const (
	groupReplication = "Replication"
	groupMemory      = "Memory"
	groupSecurity    = "Security"
	groupClock       = "Clustering"
	groupAdvanced    = "Advanced"
)

// sizeField is a `cockroach start` size knob (--cache / --max-sql-memory).
// Stored as a string so it can hold a unit ("2GiB"), a percent (".25" / "25%")
// or a ${var} placeholder resolved downstream against the node RAM at deploy
// time. CockroachDB itself accepts both fractions and byte sizes here.
func sizeField(name, def, group, title string) *schemapb.StrB {
	return schemapb.Str(name).Default(def).Group(group).Title(title)
}

// clusterSection is the pure CockroachDB engine configuration. These map to
// `cockroach start` flags (--cache, --max-sql-memory, --insecure, --max-offset)
// and the default zone replication factor (a CLUSTER SETTING / zone config),
// plus a free-form CLUSTER SETTING escape hatch. Whether nodes are colocated,
// their ports and the gossip peer list are placement/deployment concerns owned
// by the cluster schema — only the LOGICAL intent ("use gossip --join") lives
// here.
func clusterSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldCluster,
		// --- Replication ---
		schemapb.Int32(FieldReplicationFactor).In(ReplicationFactorValues...).
			Default(ReplicationFactor3).Group(groupReplication).
			Title("Default replication factor").
			Desc("Replicas per range in the default zone config. 1 = single node; "+
				"3 is the production minimum (survives 1 failure); 5 survives 2. "+
				"Must be 1 when topology=single.").
			When(utils.Eq(rp(pfx, FieldTopology), TopologyCluster)),

		// --- Memory (cockroach start flags) ---
		sizeField(FieldCache, ".25", groupMemory, "--cache").
			Desc("Storage block cache. Fraction (.25 = 25% RAM), size (2GiB) or ${var}. "+
				"~25% RAM is the recommended starting point; raise for read-heavy benches."),
		sizeField(FieldMaxSQLMemory, ".25", groupMemory, "--max-sql-memory").
			Desc("Memory budget for SQL execution (sorts, hash joins). Fraction, size or "+
				"${var}. ~25% RAM recommended; cache + max-sql-memory should stay below ~75% RAM."),

		// --- Clustering / clock ---
		schemapb.Bool(FieldGossipJoin).Default(true).Group(groupClock).
			Title("Gossip --join").
			Desc("Intent: form a cluster via gossip --join (peer list wired at the cluster "+
				"layer). Forced off for single-node.").
			When(utils.Eq(rp(pfx, FieldTopology), TopologyCluster)),
		schemapb.Int32(FieldMaxOffset).Gte(0).Lte(60000).Default(500).Unit("ms").Group(groupClock).
			Title("--max-offset").
			Desc("Maximum allowed clock skew between nodes. Lowering reduces transaction "+
				"latency uncertainty but requires tight NTP; must be identical on every node."),

		// --- Security ---
		utils.StrEnum(FieldSecurity, SecurityModeValues...).Default(SecurityInsecure).
			Group(groupSecurity).Title("Security mode").
			Desc("insecure disables all TLS/auth (--insecure) — bench-only. "+
				"secure enables node + client certificates."),

		// --- Advanced escape hatch: SET CLUSTER SETTING (schemapb has no map kind) ---
		schemapb.List(FieldClusterSettings,
			schemapb.Object(FieldSetting,
				schemapb.Str(FieldKey).Required().MinLen(1).
					Title("Setting").
					Desc("e.g. kv.snapshot_rebalance.max_rate, sql.defaults.distsql."),
				schemapb.Str(FieldValue).Required().
					Title("Value").Desc("Raw value (may use ${var})."),
			),
		).Group(groupAdvanced).Title("Cluster settings").
			Desc("Post-init `SET CLUSTER SETTING key = value` overrides applied once on node0."),
	).Title("Cluster configuration")
}
