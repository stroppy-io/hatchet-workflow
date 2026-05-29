package postgres

import (
	"fmt"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// dbRules are the DB-internal cross-field rules. They reference only fields of
// THIS schema, so they validate correctly standalone (root = the postgres form).
// Paths go through rp(pfx, ...) so the schema stays embeddable. Each rule that
// touches a gated subtree leads with a short-circuiting guard.
func dbRules(pfx string) []schemapb.RuleDef {
	var (
		topology    = rp(pfx, FieldTopology)
		replicas    = rp(pfx, FieldReplicaCount)
		syncMode    = rp(pfx, FieldHA, FieldPatroni, FieldSynchronousMode)
		syncCount   = rp(pfx, FieldHA, FieldPatroni, FieldSynchronousNodeCount)
		replMode    = rp(pfx, FieldReplication, FieldMode)
		syncCommit  = rp(pfx, FieldReplication, FieldSynchronousCommit)
		walSenders  = rp(pfx, FieldReplication, FieldMaxWalSenders)
		pgMajor     = rp(pfx, FieldPgMajor)
		walCompress = rp(pfx, FieldTuning, FieldWalCompression)
	)
	return []schemapb.RuleDef{
		schemapb.Rule(
			fmt.Sprintf("%s || %s || %s <= %s",
				utils.Ne(topology, TopologyPatroniHA),
				utils.Eq(syncMode, SyncModeOff), syncCount, replicas),
			"synchronous_node_count must be <= replica_count",
		).ID("sync_count_le_replicas"),

		schemapb.Rule(
			fmt.Sprintf("%s < 1 || %s >= %s", replicas, walSenders, replicas),
			"max_wal_senders must be >= replica_count",
		).ID("wal_senders_ge_replicas"),

		schemapb.Rule(
			fmt.Sprintf("%s < 1 || %s || (%s && %s)",
				replicas, utils.Ne(replMode, ReplSync),
				utils.Ne(syncCommit, SyncCommitOff), utils.Ne(syncCommit, SyncCommitLocal)),
			"synchronous replication with synchronous_commit off/local does not wait for standbys",
		).ID("sync_needs_waiting_commit").Warn(),

		schemapb.Rule(
			fmt.Sprintf("%s || %s == 0", utils.Ne(topology, TopologySingle), replicas),
			"topology=single must have replica_count == 0",
		).ID("single_no_replicas"),

		schemapb.Rule(
			fmt.Sprintf("%s >= %d || (%s && %s)",
				pgMajor, PgMajor15,
				utils.Ne(walCompress, WalCompressionLz4), utils.Ne(walCompress, WalCompressionZstd)),
			"wal_compression lz4/zstd requires PostgreSQL 15+",
		).ID("wal_compression_version"),
	}
}
