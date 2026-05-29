package mysql

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// routingSection holds only the DB/workload routing INTENT. The proxy
// implementation, listen ports and backend health-check wiring are deployment
// concerns wired at the cluster layer (they need machine/provider knowledge).
func routingSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldRouting,
		schemapb.Bool(FieldReadWriteSplit).Default(true).
			When(utils.Gte(rp(pfx, FieldReplicaCount), 1)).
			Title("Read/write split").
			Desc("Intent: route reads to replicas/readers. The cluster layer wires endpoints."),
		utils.StrEnum(FieldWorkloadConnectTarget, WorkloadConnectTargetValues...).
			Default(ConnectPrimary).
			Title("Workload connect target").
			Desc("Which logical endpoint the benchmark connects to."),
	).Title("Routing intent")
}
