package postgres

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
				"primary_replicas: primary + N streaming replicas, manual failover. " +
				"patroni_ha: Patroni-managed cluster with a DCS and automatic failover."),

		schemapb.Int32(FieldPgMajor).In(PgMajorValues...).Default(PgMajor16).Required().
			Title("PostgreSQL major version"),

		utils.StrEnum(FieldPgFlavor, PgFlavorValues...).Default(FlavorVanilla).
			Title("Distribution").
			Desc("vanilla PostgreSQL or a fork; citus/timescaledb swap their tuning profile."),

		schemapb.Int32(FieldReplicaCount).Gte(0).Lte(64).Default(0).
			Title("Replica count").
			Desc("Number of streaming standbys (logical). Ignored when topology=single. " +
				"Backing machines are allocated by the cluster schema."),
	}
}
