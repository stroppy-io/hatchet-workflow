package postgres

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// haSection is the Patroni + DCS configuration (logical). Active only for
// patroni_ha. The etcd/DCS tier as MACHINES is derived at the cluster layer.
func haSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldHA,
		schemapb.Object(FieldDCS,
			utils.StrEnum(FieldType, DCSTypeValues...).Default(DCSEtcd).
				Title("DCS type").Desc("Distributed configuration store backing Patroni."),
			schemapb.Int32(FieldClusterSize).In(ClusterSizeValues...).Default(ClusterSize3).
				Title("DCS cluster size").Desc("Odd for quorum; 1 = no DCS HA (test only)."),
		).Title("DCS"),

		schemapb.Object(FieldPatroni,
			schemapb.Int32(FieldTTL).Gte(10).Lte(60).Default(30).Unit("s").Title("Leader lock TTL"),
			schemapb.Int32(FieldLoopWait).Gte(5).Lte(20).Default(10).Unit("s").Title("Loop wait"),
			schemapb.Int32(FieldRetryTimeout).Gte(5).Lte(30).Default(10).Unit("s").Title("Retry timeout"),
			schemapb.Int64(FieldMaxLagOnFailover).Gte(0).Default(1048576).Unit("bytes").
				Title("Max lag on failover"),
			schemapb.Int32(FieldMasterStartTimeout).Gte(0).Default(300).Unit("s").
				Title("Master start timeout"),
			utils.StrEnum(FieldSynchronousMode, SynchronousModeValues...).Default(SyncModeOff).
				Title("Synchronous mode"),
			schemapb.Int32(FieldSynchronousNodeCount).Gte(1).Default(1).
				When(utils.Ne(rp(pfx, FieldHA, FieldPatroni, FieldSynchronousMode), SyncModeOff)).
				Title("Synchronous node count").
				Desc("Number of synchronous standbys. (Bound to replica_count at the cluster layer.)"),
		).Title("Patroni"),
	).When(utils.Eq(rp(pfx, FieldTopology), TopologyPatroniHA)).Title("High Availability")
}
