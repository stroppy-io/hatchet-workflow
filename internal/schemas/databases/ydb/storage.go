package ydb

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// storageSection is the LOGICAL blob-storage layout (pure DB). It maps to the
// static config.yaml: static_erasure, domains_config.storage_pool_types[],
// pool_config (box_id, erasure_species, kind, vdisk_kind, pdisk_filter) and
// state_storage.nto_select.
//
// IMPORTANT — what is NOT here (deploy/cluster concerns):
//   - pdisk SIZE: auto-calculated downstream from real disk capacity
//     (ceil(rawGB/93)*93 etc.); we keep only the logical media kind here.
//   - the provider disk CLASS (network-ssd, gp3, ...) and the physical device
//     paths (/dev/disk/by-partlabel/...): owned by provider/cluster.
//   - the cross-field rule "mirror-3-dc + disk fail domains => >=3 storage
//     nodes across 3 DCs": enforced at the cluster layer. Here we only do
//     single-field range checks.
func storageSection() schemapb.FieldDef {
	return schemapb.Object(FieldStorage,
		utils.StrEnum(FieldFaultTolerance, FaultToleranceValues...).
			Default(FaultToleranceNone).
			Title("Fault tolerance (erasure)").
			Desc("static_erasure / pool erasure_species. none: no redundancy (test). "+
				"block-4-2: 1.5x overhead, single data center (>=8 nodes). "+
				"mirror-3-dc: 3x overhead, three data centers (>=3 nodes)."),

		utils.StrEnum(FieldFailureDomainType, FailureDomainTypeValues...).
			Default(FailureDomainDefault).
			Title("Failure domain type").
			Desc("\"\": host-level fail domains (default). disk: each pdisk is its own "+
				"fail domain (mirror-3-dc-on-single-host test layouts)."),

		utils.StrEnum(FieldPoolMediaKind, PoolMediaKindValues...).
			Default(PoolMediaSSD).
			Title("Default pool media kind").
			Desc("Logical media family for the default storage pool (domain "+
				"storage_pool_types[].kind + pdisk_filter property type). Provider disk "+
				"CLASS and sizing are mapped at the cluster layer."),

		utils.StrEnum(FieldVDiskKind, VDiskKindValues...).
			Default(VDiskKindDefault).
			Title("VDisk kind").
			Desc("pool_config.vdisk_kind."),

		schemapb.Int32(FieldStorageGroups).Gte(0).Lte(1024).Default(1).
			Title("Storage groups").
			Desc("Number of storage groups created for the database pool "+
				"(`database create <pool>:<groups>`). 0 means resolve a default downstream."),

		schemapb.Int32(FieldBoxID).Gte(1).Default(1).
			Title("Box ID").
			Desc("pool_config.box_id grouping for pdisks."),

		schemapb.Int32(FieldStateStorageNTo).Gte(0).Lte(9).Default(0).
			Title("State storage nto_select").
			Desc("state_storage ring nto_select (quorum width). 0 => derived downstream "+
				"from node count and erasure (e.g. 5 for block-4-2, 9 for mirror-3-dc)."),
	).Title("Storage layout")
}
