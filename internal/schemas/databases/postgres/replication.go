package postgres

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// replicationSection covers streaming replication knobs (pure DB). Active when
// there is at least one replica.
func replicationSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldReplication,
		utils.StrEnum(FieldMode, ReplicationModeValues...).Default(ReplAsync).
			Title("Replication mode"),
		utils.StrEnum(FieldSynchronousCommit, SynchronousCommitValues...).Default(SyncCommitOn).
			Title("synchronous_commit"),
		utils.StrEnum(FieldSyncMethod, SyncMethodValues...).Default(SyncMethodFirst).
			When(utils.Eq(rp(pfx, FieldReplication, FieldMode), ReplSync)).
			Title("Sync method").Desc("FIRST n (priority) or ANY n (quorum) synchronous standbys."),
		utils.StrEnum(FieldWalLevel, WalLevelValues...).Default(WalLevelReplica).Title("wal_level"),
		schemapb.Int32(FieldMaxWalSenders).Gte(0).Lte(64).Default(10).Title("max_wal_senders"),
		schemapb.Int32(FieldMaxReplicationSlots).Gte(0).Lte(64).Default(10).Title("max_replication_slots"),
		schemapb.Bool(FieldUseReplicationSlots).Default(true).Title("Use replication slots"),
		schemapb.Bool(FieldHotStandbyFeedback).Default(false).Title("hot_standby_feedback"),
		schemapb.Bool(FieldCascading).Default(false).
			When(utils.Gte(rp(pfx, FieldReplicaCount), 2)).Title("Cascading replication"),
		schemapb.Int64(FieldWalKeepSize).Gte(0).Default(0).Unit("MB").Title("wal_keep_size"),
	).When(utils.Gte(rp(pfx, FieldReplicaCount), 1)).Title("Replication")
}
