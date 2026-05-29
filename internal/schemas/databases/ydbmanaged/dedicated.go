package ydbmanaged

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// dedicatedSection holds the dedicated-cluster YDB knobs (maps to
// yandex_ydb_database_dedicated). Active only when type == dedicated.
//
// The scale policy is a nested discriminator (scale.scale_type fixed|auto)
// mirroring the terraform scale_policy.fixed XOR scale_policy.auto contract:
//   - fixed: node_count drives scale_policy.fixed.size.
//   - auto:  min_size/max_size/cpu_utilization_percent drive scale_policy.auto.
//
// storage_groups + storage_type map to storage_config.group_count +
// storage_type_id.
func dedicatedSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldDedicated,
		utils.StrEnum(FieldResourcePresetID, ResourcePresetIDValues...).
			Default(PresetMedium).Required().
			Title("Resource preset").
			Desc("Hardware class for each node (cores/RAM). The picker filters this "+
				"list by compute_type. The value is the terraform resource_preset_id."),

		schemapb.Object(FieldScale,
			utils.StrEnum(FieldScaleType, ScaleTypeValues...).Default(ScaleFixed).Required().
				Title("Scale policy").
				Desc("fixed: a constant node count. auto: autoscale between min/max on CPU."),

			// fixed: node_count -> scale_policy.fixed.size
			schemapb.Int32(FieldNodeCount).Gte(1).Lte(64).Default(1).
				When(utils.Eq(rp(pfx, FieldDedicated, FieldScale, FieldScaleType), ScaleFixed)).
				Title("Node count").
				Desc("Number of database nodes (scale_policy.fixed.size)."),

			// auto: min/max/cpu% -> scale_policy.auto
			schemapb.Int32(FieldMinSize).Gte(1).Lte(64).Default(1).
				When(utils.Eq(rp(pfx, FieldDedicated, FieldScale, FieldScaleType), ScaleAuto)).
				Title("Min size").
				Desc("Minimum node count for autoscaling (>= 1)."),
			schemapb.Int32(FieldMaxSize).Gte(1).Lte(64).Default(3).
				When(utils.Eq(rp(pfx, FieldDedicated, FieldScale, FieldScaleType), ScaleAuto)).
				Title("Max size").
				Desc("Maximum node count for autoscaling (>= min_size)."),
			schemapb.Int32(FieldCPUUtilizationPercent).Gte(1).Lte(100).Default(70).Unit("%").
				When(utils.Eq(rp(pfx, FieldDedicated, FieldScale, FieldScaleType), ScaleAuto)).
				Title("Target CPU utilization").
				Desc("Target average CPU that drives autoscaling (1..100)."),
		).
			// Enforce max_size >= min_size on the auto branch (mirrors the terraform
			// validation). Only checked when the auto branch is active.
			Rule(schemapb.Rule(
				rp(pfx, FieldDedicated, FieldScale, FieldScaleType)+" != '"+ScaleAuto+"' || "+
					rp(pfx, FieldDedicated, FieldScale, FieldMaxSize)+" >= "+
					rp(pfx, FieldDedicated, FieldScale, FieldMinSize),
				"auto scale max_size must be >= min_size",
			).ID("ydb_managed_autoscale_range")).
			Title("Scale policy"),

		schemapb.Int32(FieldStorageGroups).Gte(1).Lte(256).Default(1).
			Title("Storage groups").
			Desc("Number of storage groups (storage_config.group_count). More groups = "+
				"more throughput and capacity."),

		utils.StrEnum(FieldStorageType, StorageTypeIDValues...).Default(StorageSSD).
			Title("Storage type").
			Desc("Storage media (storage_config.storage_type_id). ssd for latency-sensitive "+
				"workloads, hdd for bulk capacity."),
	).When(utils.Eq(rp(pfx, FieldType), TypeDedicated)).Title("Dedicated")
}
