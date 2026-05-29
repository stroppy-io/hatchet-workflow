package mariadb

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
				"galera: synchronous multi-primary Galera cluster (wsrep)."),

		utils.StrEnum(FieldVersion, MariaDBVersionValues...).Default(Version114).Required().
			Title("MariaDB version").
			Desc("10.11 (LTS) or 11.4 (LTS)."),

		schemapb.Int32(FieldReplicaCount).Gte(0).Lte(64).Default(0).
			Title("Replica count").
			Desc("Number of secondaries (logical). Ignored when topology=single. " +
				"For galera this is the count of additional cluster nodes (3 total = 1+2 " +
				"recommended for quorum). Backing machines are allocated by the cluster schema."),
	}
}
