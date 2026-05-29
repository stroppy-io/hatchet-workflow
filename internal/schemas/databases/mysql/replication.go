package mysql

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// replicationSection covers the replication knobs (pure DB). Active when there
// is at least one replica OR the topology is group_replication.
//
// XOR note: replication.mode is async XOR semi_sync XOR group_replication. The
// cross-field rule that mode must agree with topology (group_replication topology
// <=> mode=group_replication) lives at the CLUSTER layer — do NOT add it here.
func replicationSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldReplication,
		utils.StrEnum(FieldMode, ReplicationModeValues...).Default(ReplAsync).
			Title("Replication mode").
			Desc("XOR: async (no ack), semi_sync (primary waits for N acks), or "+
				"group_replication (Paxos group). Must agree with topology — enforced at cluster."),

		// GTID — strongly recommended for crash-safe replication & GR (mandatory for GR).
		utils.StrEnum(FieldGtidMode, GtidModeValues...).Default(GtidOn).
			Title("gtid_mode").Desc("ON is required for InnoDB Group Replication and auto-positioning."),
		schemapb.Bool(FieldEnforceGtidConsistency).Default(true).
			Title("enforce_gtid_consistency").Desc("Required ON alongside gtid_mode=ON."),

		utils.StrEnum(FieldBinlogFormat, BinlogFormatValues...).Default(BinlogRow).
			Title("binlog_format").Desc("ROW is the 8.x default and the only format valid for GR."),
		schemapb.Bool(FieldLogBin).Default(true).Title("log_bin").
			Desc("Binary logging; required for any replication."),
		schemapb.Bool(FieldLogReplicaUpdates).Default(true).Title("log_replica_updates").
			Desc("Cascade replicated events into this server's binlog (required for GR)."),

		// Parallel applier (multi-threaded replica) — big win for replica lag.
		schemapb.Int32(FieldReplicaParallelWorkers).Gte(0).Lte(1024).Default(4).
			Title("replica_parallel_workers").Desc("0 disables parallel apply (8.x: LOGICAL_CLOCK)."),
		schemapb.Bool(FieldReplicaPreserveCommitOrd).Default(true).
			Title("replica_preserve_commit_order").Desc("Required ON for crash-safe parallel apply / GR."),

		// Semi-sync knobs — only when mode=semi_sync.
		schemapb.Int32(FieldSemiSyncTimeout).Gte(0).Default(10000).Unit("ms").
			When(utils.Eq(rp(pfx, FieldReplication, FieldMode), ReplSemiSync)).
			Title("rpl_semi_sync_source_timeout").
			Desc("Fall back to async if no replica ack within this window."),
		schemapb.Int32(FieldSemiSyncWaitForReplica).Gte(1).Default(1).
			When(utils.Eq(rp(pfx, FieldReplication, FieldMode), ReplSemiSync)).
			Title("rpl_semi_sync_source_wait_for_replica_count").
			Desc("Number of replica acks required before commit."),

		// Group Replication sub-object — only when mode=group_replication.
		schemapb.Object(FieldGroupReplication,
			schemapb.Str(FieldGroupName).Default("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa").
				Title("group_replication_group_name").
				Desc("UUID identifying the group. (Bootstrapped at the cluster layer.)"),
			utils.StrEnum(FieldGroupReplMode, GroupReplModeValues...).Default(GRSinglePrimary).
				Title("Group mode").Desc("single_primary (recommended) or multi_primary."),
			utils.StrEnum(FieldGroupConsistency, GroupReplConsistencyValues...).
				Default(GRConsistencyBeforeOnPrimary).
				Title("group_replication_consistency").
				Desc("Read/write consistency guarantee across the group."),
			schemapb.Int32(FieldGroupLocalAddressPort).Gte(1024).Lte(65535).Default(33061).
				Title("Group local address port").
				Desc("Intra-group (XCom) port. The host part is wired at the cluster layer."),
			schemapb.Int32(FieldGroupAutoRejoinTries).Gte(0).Lte(2016).Default(3).
				Title("group_replication_autorejoin_tries"),
			schemapb.Bool(FieldGroupExitStateAction).Default(true).
				Title("exit_state_action = OFFLINE_MODE").
				Desc("true: go OFFLINE_MODE on expel/unreachable; false: ABORT_SERVER."),
			schemapb.Bool(FieldGroupFlowControlMode).Default(true).
				Title("flow_control_mode = QUOTA").
				Desc("true: QUOTA throttling to bound replica lag; false: DISABLED."),
		).When(utils.Eq(rp(pfx, FieldReplication, FieldMode), ReplGroup)).
			Title("InnoDB Group Replication"),
	).When(utils.Gte(rp(pfx, FieldReplicaCount), 1)).Title("Replication")
}
