package ydbmanaged

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// topologyFields are the root discriminators of the managed-YDB database. `type`
// selects the terraform resource flavor; `compute_type` is the UI preset filter
// (oltp/olap) and is NOT forwarded to terraform; `database_path` is the YDB
// database path the workload connects to.
//
// There is NO client VM, network, SA or credential surface here — those are
// provider/cluster concerns, not part of the database-under-test config.
func topologyFields() []schemapb.FieldDef {
	return []schemapb.FieldDef{
		utils.StrEnum(FieldType, DBTypeValues...).
			Default(TypeServerless).Required().
			Title("Database type").
			Desc("Root discriminator. serverless: pay-per-request YDB database with " +
				"optional RCU throttling. dedicated: a provisioned cluster with a " +
				"resource preset and a fixed or autoscaling node group."),

		utils.StrEnum(FieldComputeType, ComputeTypeValues...).Default(ComputeOLTP).
			Title("Compute type").
			Desc("Workload class. On dedicated this filters the resource-preset picker " +
				"(oltp/olap); it is a UI hint and is not sent to terraform."),

		schemapb.Str(FieldDatabasePath).Default("/Root/testdb").
			Title("Database path").
			Desc("YDB database path the workload connects to " +
				"(grpcs://<host>:<port>/?database=<path>). Often filled from terraform " +
				"output at run time."),
	}
}
