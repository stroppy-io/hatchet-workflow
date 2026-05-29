package ydbmanaged

import "github.com/stroppy-io/stroppy-cloud/internal/schemas/types"

// String vocabulary for the Yandex Managed Service for YDB database schema:
// enum values, field names, and the expr paths used by its (DB-internal) gates.
//
// This schema models the DATABASE-UNDER-TEST config the user chooses for a
// Yandex Managed YDB offering. It is intentionally NOT the full provider/cluster
// surface: the client runner VM, the terraform credentials, the network/SA
// plumbing and the runtime-filled endpoint/database_path live in the provider
// schema and the cluster (top) schema. Here we keep only the knobs that map to
// the `managed_ydb` terraform variable's database shape (serverless vs
// dedicated, scale policy, storage config).
//
// Enum value types are string aliases so they pass straight into
// utils.StrEnum(...).In() / .Default(); each ships a <Name>Values slice.

// =============================================================================
// Enum value types (string) + values
// =============================================================================

// DBType is the managed-YDB flavor — the root discriminator. It selects the
// terraform resource (yandex_ydb_database_serverless vs _dedicated).
type DBType = string

const (
	TypeServerless DBType = "serverless"
	TypeDedicated  DBType = "dedicated"
)

var DBTypeValues = []string{TypeServerless, TypeDedicated}

// ComputeType is the workload class. On dedicated it is purely a UI filter for
// the resource-preset picker (NOT forwarded to terraform); serverless is OLTP.
type ComputeType = string

const (
	ComputeOLTP ComputeType = "oltp"
	ComputeOLAP ComputeType = "olap"
)

var ComputeTypeValues = []string{ComputeOLTP, ComputeOLAP}

// ScaleType is the dedicated scale-policy discriminator: a fixed node count or
// an autoscaling group. Mirrors scale_policy.fixed XOR scale_policy.auto in the
// managed_ydb terraform variable.
type ScaleType = string

const (
	ScaleFixed ScaleType = "fixed"
	ScaleAuto  ScaleType = "auto"
)

var ScaleTypeValues = []string{ScaleFixed, ScaleAuto}

// ResourcePresetID is a dedicated-cluster hardware class. The value set mirrors
// YDB_MANAGED_RESOURCE_PRESETS (web/src/api/types.ts) — keep the two in sync
// when YC publishes new presets. The stored string IS the terraform
// resource_preset_id.
type ResourcePresetID = string

const (
	PresetSmallM8    ResourcePresetID = "small-m8"      // 4 cores / 8 GB   (oltp)
	PresetSmall      ResourcePresetID = "small"         // 4 cores / 16 GB  (oltp)
	PresetMedium     ResourcePresetID = "medium"        // 8 cores / 32 GB  (oltp)
	PresetMediumM64  ResourcePresetID = "medium-m64"    // 8 cores / 64 GB  (oltp)
	PresetMediumM96  ResourcePresetID = "medium-m96"    // 8 cores / 96 GB  (oltp)
	PresetLarge      ResourcePresetID = "large"         // 12 cores / 48 GB (oltp)
	PresetXLarge     ResourcePresetID = "xlarge"        // 16 cores / 64 GB (oltp)
	PresetOLTPC16    ResourcePresetID = "oltp-c16-m128" // 16 cores / 128 GB (oltp)
	PresetOLAPMedium ResourcePresetID = "olap-medium"   // 8 cores / 32 GB  (olap)
	PresetOLAPLarge  ResourcePresetID = "olap-large"    // 12 cores / 48 GB (olap)
)

var ResourcePresetIDValues = []string{
	PresetSmallM8, PresetSmall, PresetMedium, PresetMediumM64, PresetMediumM96,
	PresetLarge, PresetXLarge, PresetOLTPC16, PresetOLAPMedium, PresetOLAPLarge,
}

// StorageTypeID is the dedicated database storage media (terraform
// storage_config.storage_type_id). ssd is the default; hdd is the rotational tier.
type StorageTypeID = string

const (
	StorageSSD StorageTypeID = "ssd"
	StorageHDD StorageTypeID = "hdd"
)

var StorageTypeIDValues = []string{StorageSSD, StorageHDD}

// =============================================================================
// Field names
// =============================================================================

const (
	// root discriminators
	FieldType         = types.Field("type")
	FieldComputeType  = types.Field("compute_type")
	FieldDatabasePath = types.Field("database_path")

	// serverless (When type == serverless)
	FieldServerless       = types.Field("serverless")
	FieldThrottlingRCUs   = types.Field("throttling_rcus")
	FieldProvisionedRCU   = types.Field("provisioned_rcu")
	FieldStorageSizeLimit = types.Field("storage_size_limit")

	// dedicated (When type == dedicated)
	FieldDedicated        = types.Field("dedicated")
	FieldResourcePresetID = types.Field("resource_preset_id")
	FieldStorageGroups    = types.Field("storage_groups")
	FieldStorageType      = types.Field("storage_type")

	// dedicated scale policy (nested discriminator)
	FieldScale                 = types.Field("scale")
	FieldScaleType             = types.Field("scale_type")
	FieldNodeCount             = types.Field("node_count")
	FieldMinSize               = types.Field("min_size")
	FieldMaxSize               = types.Field("max_size")
	FieldCPUUtilizationPercent = types.Field("cpu_utilization_percent")
)
