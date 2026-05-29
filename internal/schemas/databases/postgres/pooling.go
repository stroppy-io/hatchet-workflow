package postgres

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// poolingSection configures an optional connection pooler (CONFIG only).
// Whether the pooler is colocated or on a dedicated machine is a placement
// decision owned by the cluster schema.
func poolingSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldPooling,
		schemapb.Bool(FieldUsePooler).Default(false).Title("Use connection pooler"),
		schemapb.Object(FieldPooler,
			utils.StrEnum(FieldType, PoolerTypeValues...).Default(PoolerPgBouncer).Title("Pooler"),
			utils.StrEnum(FieldPoolMode, PoolModeValues...).Default(PoolModeTransaction).
				Title("Pool mode").
				Desc("transaction/statement disable session-level features "+
					"(prepared statements, SET, advisory locks, LISTEN/NOTIFY)."),
			schemapb.Int32(FieldMaxClientConn).Gte(1).Default(1000).Title("max_client_conn"),
			schemapb.Int32(FieldDefaultPoolSize).Gte(1).Default(20).Title("default_pool_size"),
			utils.StrEnum(FieldAuthType, PoolerAuthTypeValues...).Default(PoolerAuthSCRAM).Title("Auth type"),
		).When(utils.IsTrue(rp(pfx, FieldPooling, FieldUsePooler))).Title("Pooler"),
	).Title("Connection pooling")
}
