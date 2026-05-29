package mariadb

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// replicationSection covers the replication knobs (pure DB). Active when there
// is at least one replica OR the topology is galera.
//
// XOR note: replication.mode is async XOR semi_sync XOR galera. The cross-field
// rule that mode must agree with topology (galera topology <=> mode=galera) lives
// at the CLUSTER layer — do NOT add it here.
func replicationSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldReplication,
		utils.StrEnum(FieldMode, ReplicationModeValues...).Default(ReplAsync).
			Title("Replication mode").
			Desc("XOR: async (no ack), semi_sync (primary waits for N acks), or "+
				"galera (synchronous certification-based). Must agree with topology — "+
				"enforced at cluster."),

		// MariaDB GTID is implicit; strict mode enforces consistent GTID ordering.
		schemapb.Bool(FieldGtidStrictMode).Default(true).
			Title("gtid_strict_mode").Desc("Reject out-of-order GTID writes; recommended for crash-safe replication."),
		schemapb.Int64(FieldGtidDomainID).Gte(0).Lte(4294967295).Default(0).
			Title("gtid_domain_id").Desc("Replication domain id; keep distinct per writer in multi-source."),

		utils.StrEnum(FieldBinlogFormat, BinlogFormatValues...).Default(BinlogRow).
			Title("binlog_format").Desc("ROW is required for Galera."),
		schemapb.Bool(FieldLogBin).Default(true).Title("log_bin").
			Desc("Binary logging; required for async/semi-sync replication."),
		schemapb.Bool(FieldLogReplicaUpdates).Default(true).Title("log_slave_updates").
			Desc("Cascade replicated events into this server's binlog."),

		// Parallel replication.
		schemapb.Int32(FieldReplicaParallelThreads).Gte(0).Lte(1024).Default(4).
			Title("slave_parallel_threads").Desc("0 disables parallel apply."),
		utils.StrEnum(FieldReplicaParallelMode, []string{"optimistic", "conservative", "aggressive", "minimal", "none"}...).
			Default("optimistic").Title("slave_parallel_mode"),

		// Semi-sync knobs — only when mode=semi_sync.
		schemapb.Int32(FieldSemiSyncTimeout).Gte(0).Default(10000).Unit("ms").
			When(utils.Eq(rp(pfx, FieldReplication, FieldMode), ReplSemiSync)).
			Title("rpl_semi_sync_master_timeout").
			Desc("Fall back to async if no replica ack within this window."),
		schemapb.Bool(FieldSemiSyncWaitNoReplica).Default(true).
			When(utils.Eq(rp(pfx, FieldReplication, FieldMode), ReplSemiSync)).
			Title("rpl_semi_sync_master_wait_no_slave"),

		// Galera sub-object — only when mode=galera.
		schemapb.Object(FieldGalera,
			schemapb.Str(FieldClusterName).Default("stroppy_galera").
				Title("wsrep_cluster_name").Desc("Unique cluster name shared by all nodes."),
			utils.StrEnum(FieldSstMethod, WsrepSstMethodValues...).Default(SstMariabackup).
				Title("wsrep_sst_method").Desc("State Snapshot Transfer method; mariabackup is non-blocking."),
			// wsrep_slave_threads: rule of thumb ~4 per CPU core.
			schemapb.Int32(FieldSlaveThreads).Gte(1).Lte(512).Default(4).
				Title("wsrep_slave_threads").Desc("Apply threads; ~4 per CPU core for faster catch-up."),
			schemapb.Int32(FieldLocalAddressPort).Gte(1024).Lte(65535).Default(4567).
				Title("wsrep gcomm port").
				Desc("Group-communication (gcomm) port. The host part is wired at the cluster layer. "+
					"SST uses 4444, IST uses 4568."),
			schemapb.Str(FieldGcacheSize).Default("128M").
				Title("gcache.size").Desc("Galera write-set cache; size for the IST window."),
			schemapb.Str(FieldProviderOptions).Default("").
				Title("wsrep_provider_options").
				Desc("Raw semicolon-separated provider tuning (e.g. gcache.size=1G;gcs.fc_limit=256)."),
		).When(utils.Eq(rp(pfx, FieldReplication, FieldMode), ReplGalera)).
			Title("Galera"),
	).When(utils.Gte(rp(pfx, FieldReplicaCount), 1)).Title("Replication")
}
