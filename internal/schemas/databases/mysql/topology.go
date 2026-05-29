package mysql

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// topologyFields are the root discriminators of the logical topology. There is
// NO machine/provider/count-of-instances here — only the logical shape and the
// replica count. The cluster schema derives instances/machines from these.
func topologyFields() []schemapb.FieldDef {
	return []schemapb.FieldDef{
		utils.StrEnum(FieldTopology, TopologyValues...).
			Default(TopologySingle).Required().
			Title("Topology").
			Desc("Cluster shape. Root discriminator. single: one node. " +
				"primary_replicas: primary + N async/semi-sync replicas. " +
				"group_replication: InnoDB Group Replication (Paxos-based, self-healing)."),

		utils.StrEnum(FieldVersion, MySQLVersionValues...).Default(Version84).Required().
			Title("MySQL version").
			Desc("8.0 (LTS) or 8.4 (LTS). 8.4 defaults to caching_sha2_password and " +
				"innodb_dedicated_server, and uses innodb_redo_log_capacity."),

		schemapb.Int32(FieldReplicaCount).Gte(0).Lte(64).Default(0).
			Title("Replica count").
			Desc("Number of secondaries (logical). Ignored when topology=single. " +
				"For group_replication this is the count of additional group members. " +
				"Backing machines are allocated by the cluster schema."),
	}
}
