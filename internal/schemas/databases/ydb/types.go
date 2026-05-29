package ydb

import "github.com/stroppy-io/stroppy-cloud/internal/schemas/types"

// String vocabulary for the PURE self-hosted YDB database schema: enum values,
// field names, and the expr paths used by its (DB-internal) gates. This schema is
// provider-agnostic — it knows nothing about machines, disks (hardware), VM
// sizing, providers or placement. Those live in the provider schema and the
// cluster (top) schema, which also owns every cross-field rule (e.g.
// mirror-3-dc requiring >=3 storage nodes across 3 datacenters, and pdisk SIZE
// auto-calculation against real disk capacity).
//
// Enum value types are string aliases so they pass straight into
// utils.StrEnum(...).In() / .Default(); each ships a <Name>Values slice.

// =============================================================================
// Enum value types (string / int) + values
// =============================================================================

// Topology is the cluster shape — the root discriminator. YDB always has a
// static storage tier; the question is whether the dynamic database/compute
// nodes are co-located on the storage VMs (combined) or run on separate VMs
// (split). This is a LOGICAL distinction; the actual VM allocation + the
// combined-mode memory-halving are resolved at the cluster/deploy layer.
type Topology = string

const (
	// TopologyCombined runs ydbd-storage and ydbd-database on the same nodes.
	TopologyCombined Topology = "combined"
	// TopologySplit runs the dynamic database/compute nodes on separate nodes.
	TopologySplit Topology = "split"
)

var TopologyValues = []string{TopologyCombined, TopologySplit}

// FaultTolerance is the static blob-storage erasure species (`static_erasure`).
type FaultTolerance = string

const (
	// FaultToleranceNone is no redundancy (single failure domain) — test only.
	FaultToleranceNone FaultTolerance = "none"
	// FaultToleranceBlock42 is block-4-2 (1.5x overhead) — single data center.
	FaultToleranceBlock42 FaultTolerance = "block-4-2"
	// FaultToleranceMirror3DC is mirror-3-dc (3x overhead) — three data centers.
	FaultToleranceMirror3DC FaultTolerance = "mirror-3-dc"
)

var FaultToleranceValues = []string{FaultToleranceNone, FaultToleranceBlock42, FaultToleranceMirror3DC}

// FailureDomainType is how fail domains are derived for the storage groups.
// "" (default) lets YDB pick host-level domains; "disk" makes each pdisk its
// own fail domain (used by mirror-3-dc-on-one-host test layouts).
type FailureDomainType = string

const (
	FailureDomainDefault FailureDomainType = ""
	FailureDomainDisk    FailureDomainType = "disk"
)

var FailureDomainTypeValues = []string{FailureDomainDefault, FailureDomainDisk}

// PoolMediaKind is the LOGICAL storage pool media kind written to the domain
// storage_pool_types[].kind and pdisk_filter. It is NOT a provider disk class
// (gp3/network-ssd/...): that mapping lives in the provider/cluster schema. It
// only tells YDB which media family backs the default pool.
type PoolMediaKind = string

const (
	PoolMediaSSD  PoolMediaKind = "ssd"
	PoolMediaNVMe PoolMediaKind = "nvme"
	PoolMediaROT  PoolMediaKind = "rot" // rotational / HDD
)

var PoolMediaKindValues = []string{PoolMediaSSD, PoolMediaNVMe, PoolMediaROT}

// VDiskKind is the domain pool_config vdisk_kind.
type VDiskKind = string

const (
	VDiskKindDefault VDiskKind = "Default"
	VDiskKindLog     VDiskKind = "Log"
)

var VDiskKindValues = []string{VDiskKindDefault, VDiskKindLog}

// ActorNodeType drives actor_system_config auto-tuning per node role.
type ActorNodeType = string

const (
	ActorNodeStorage ActorNodeType = "STORAGE"
	ActorNodeCompute ActorNodeType = "COMPUTE"
	ActorNodeHybrid  ActorNodeType = "HYBRID"
)

var ActorNodeTypeValues = []string{ActorNodeStorage, ActorNodeCompute, ActorNodeHybrid}

// =============================================================================
// Field names
// =============================================================================

const (
	// root
	FieldTopology          = types.Field("topology")
	FieldStorageNodeCount  = types.Field("storage_node_count")
	FieldDatabaseNodeCount = types.Field("database_node_count")
	FieldDatabasePath      = types.Field("database_path")
	FieldTenant            = types.Field("tenant")

	// storage (logical blob-storage layout)
	FieldStorage           = types.Field("storage")
	FieldFaultTolerance    = types.Field("fault_tolerance")
	FieldFailureDomainType = types.Field("failure_domain_type")
	FieldStorageGroups     = types.Field("storage_groups")
	FieldPoolMediaKind     = types.Field("pool_media_kind")
	FieldVDiskKind         = types.Field("vdisk_kind")
	FieldBoxID             = types.Field("box_id")
	FieldStateStorageNTo   = types.Field("state_storage_nto_select")

	// protocols / endpoints surface
	FieldProtocols   = types.Field("protocols")
	FieldGRPCPort    = types.Field("grpc_port")
	FieldEnablePgI   = types.Field("enable_pgwire")
	FieldPgWirePort  = types.Field("pgwire_port")
	FieldInterconStg = types.Field("interconnect_port_storage")
	FieldInterconDb  = types.Field("interconnect_port_database")
	FieldMonPortStg  = types.Field("mon_port_storage")
	FieldMonPortDb   = types.Field("mon_port_database")

	// config (grouped logical config: memory / actor system / storage / database)
	FieldConfig = types.Field("config")

	// config — memory_controller_config
	FieldMemHardLimit       = types.Field("memory_hard_limit")
	FieldSharedCacheMinPct  = types.Field("shared_cache_min_percent")
	FieldSharedCacheMaxPct  = types.Field("shared_cache_max_percent")
	FieldQueryExecLimitPct  = types.Field("query_execution_limit_percent")
	FieldEnforceTokenAuth   = types.Field("enforce_user_token_requirement")
	FieldEnableQueryService = types.Field("enable_query_service")

	// config — actor_system_config
	FieldActorAutoConfig    = types.Field("actor_auto_config")
	FieldActorStorageType   = types.Field("actor_node_type_storage")
	FieldActorComputeType   = types.Field("actor_node_type_database")
	FieldActorSystemThreads = types.Field("actor_system_threads")
	FieldActorUserThreads   = types.Field("actor_user_threads")
	FieldActorBatchThreads  = types.Field("actor_batch_threads")
	FieldActorICThreads     = types.Field("actor_ic_threads")

	// config — escape hatch
	FieldStorageParams  = types.Field("storage_params")
	FieldDatabaseParams = types.Field("database_params")
	FieldParam          = types.Field("param")
	FieldKey            = types.Field("key")
	FieldValue          = types.Field("value")
)
