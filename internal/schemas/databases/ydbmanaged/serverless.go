package ydbmanaged

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// serverlessSection holds the serverless YDB knobs (maps to
// yandex_ydb_database_serverless). Active only when type == serverless.
//
// throttling_rcus caps request units per second: 0 means unlimited (the
// terraform module omits the throttling block when it is 0). provisioned_rcu
// pre-allocates capacity to avoid cold-start throttling; 0 = none.
// storage_size_limit caps the database storage in GB; 0 = unlimited (provider
// default quota).
func serverlessSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldServerless,
		schemapb.Int64(FieldThrottlingRCUs).Gte(0).Default(0).Unit("RCU/s").
			Title("Throttling RCUs").
			Desc("Per-second request-unit cap. 0 = unlimited (no throttling block emitted)."),

		schemapb.Int64(FieldProvisionedRCU).Gte(0).Default(0).Unit("RCU/s").
			Title("Provisioned RCUs").
			Desc("Pre-allocated request units to avoid cold-start throttling. 0 = none."),

		schemapb.Int64(FieldStorageSizeLimit).Gte(0).Default(0).Unit("GB").
			Title("Storage size limit").
			Desc("Maximum database storage. 0 = unlimited (provider default quota)."),
	).When(utils.Eq(rp(pfx, FieldType), TypeServerless)).Title("Serverless")
}
