package ydb

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

const (
	groupMemory   = "Memory & limits"
	groupSecurity = "Security"
	groupActor    = "Actor system"
	groupAdvanced = "Advanced"
)

// configSection is the grouped LOGICAL ydbd config (pure DB). Fields are flat
// inside one object and bucketed for the UI by Group() (schemapb has no nested
// sections). Sizes/limits are kept logical: percentages where YDB supports them,
// and a string memory hard-limit that may hold a unit ("16GB"), a percent
// ("85%") or a ${var} placeholder resolved downstream.
//
// NOTE (deploy concern): for combined topology the effective memory hard limit
// per daemon is HALVED at the cluster/deploy layer (two daemons on one node);
// we do NOT bake that here — only the logical target.
func configSection() schemapb.FieldDef {
	return schemapb.Object(FieldConfig,
		// --- Memory & limits (memory_controller_config) ---
		schemapb.Str(FieldMemHardLimit).Default("85%").Group(groupMemory).
			Title("Memory hard limit").
			Desc("memory_controller_config.hard_limit_bytes. Accepts a size (\"16GB\"), "+
				"a percent of node RAM (\"85%\") or a ${var}; resolved downstream "+
				"(and halved per-daemon for combined topology)."),
		schemapb.Int32(FieldSharedCacheMinPct).Gte(0).Lte(100).Default(10).Unit("%").
			Group(groupMemory).Title("Shared cache min percent").
			Desc("memory_controller_config.shared_cache_min_percent."),
		schemapb.Int32(FieldSharedCacheMaxPct).Gte(0).Lte(100).Default(30).Unit("%").
			Group(groupMemory).Title("Shared cache max percent").
			Desc("memory_controller_config.shared_cache_max_percent."),
		schemapb.Int32(FieldQueryExecLimitPct).Gte(0).Lte(100).Default(25).Unit("%").
			Group(groupMemory).Title("Query execution limit percent").
			Desc("memory_controller_config.query_execution_limit_percent."),

		// --- Security ---
		schemapb.Bool(FieldEnforceTokenAuth).Default(false).Group(groupSecurity).
			Title("Enforce user token requirement").
			Desc("security_config.enforce_user_token_requirement. Off for bench/insecure."),
		schemapb.Bool(FieldEnableQueryService).Default(true).Group(groupSecurity).
			Title("Enable query service").
			Desc("Enable the table/query service (table_service_config / query_service_config)."),

		// --- Actor system (actor_system_config) ---
		schemapb.Bool(FieldActorAutoConfig).Default(true).Group(groupActor).
			Title("Actor system auto-config").
			Desc("actor_system_config.use_auto_config. When on, thread pools below are ignored "+
				"and tuned from cpu_count downstream."),
		utils.StrEnum(FieldActorStorageType, ActorNodeTypeValues...).Default(ActorNodeStorage).
			Group(groupActor).Title("Actor node type (storage)").
			Desc("actor_system_config.node_type for storage nodes (auto-config)."),
		utils.StrEnum(FieldActorComputeType, ActorNodeTypeValues...).Default(ActorNodeCompute).
			Group(groupActor).Title("Actor node type (database)").
			Desc("actor_system_config.node_type for database/compute nodes (auto-config)."),
		schemapb.Int32(FieldActorSystemThreads).Gte(1).Lte(256).Default(2).Group(groupActor).
			Title("System pool threads").Desc("Manual actor_system_config System executor threads."),
		schemapb.Int32(FieldActorUserThreads).Gte(1).Lte(256).Default(3).Group(groupActor).
			Title("User pool threads").Desc("Manual actor_system_config User executor threads."),
		schemapb.Int32(FieldActorBatchThreads).Gte(1).Lte(256).Default(2).Group(groupActor).
			Title("Batch pool threads").Desc("Manual actor_system_config Batch executor threads."),
		schemapb.Int32(FieldActorICThreads).Gte(1).Lte(256).Default(1).Group(groupActor).
			Title("IC pool threads").Desc("Manual actor_system_config IC (interconnect) executor threads."),

		// --- Advanced escape hatch (schemapb has no map kind) ---
		schemapb.List(FieldStorageParams,
			schemapb.Object(FieldParam,
				schemapb.Str(FieldKey).Required().MinLen(1),
				schemapb.Str(FieldValue).Required(),
			),
		).Group(groupAdvanced).Title("Extra storage-node config").
			Desc("Raw config.yaml key/value overrides for storage nodes (values may use ${var})."),
		schemapb.List(FieldDatabaseParams,
			schemapb.Object(FieldParam,
				schemapb.Str(FieldKey).Required().MinLen(1),
				schemapb.Str(FieldValue).Required(),
			),
		).Group(groupAdvanced).Title("Extra database-node config").
			Desc("Raw config key/value overrides for dynamic database nodes (values may use ${var})."),
	).Title("Server config")
}
