package picodata

import "github.com/stroppy-io/stroppy-cloud/internal/schemas/types"

// String vocabulary for the PURE Picodata database schema: enum values, field
// names, and the expr paths used by its (DB-internal) gates. This schema is
// provider-agnostic — it knows nothing about machines, disks, providers or
// placement. Those live in the provider schema and the cluster (top) schema.
//
// Enum value types are string aliases so they pass straight into
// utils.StrEnum(...).In() / .Default(); each ships a <Name>Values slice.

// =============================================================================
// Enum value types (string) + values
// =============================================================================

// Topology is the cluster shape — the root discriminator.
//
// Picodata is always a distributed, shared-nothing database; even a "single"
// deployment is a one-instance cluster. The discriminator here selects between a
// single-instance form (1 replicaset, no sharding spread) and a multi-instance
// cluster (raft + vshard across replicasets/tiers).
type Topology = string

const (
	TopologySingle  Topology = "single"
	TopologyCluster Topology = "cluster"
)

var TopologyValues = []string{TopologySingle, TopologyCluster}

// LogLevel is instance.log.level. Picodata/Tarantool ordering, least→most verbose.
type LogLevel = string

const (
	LogFatal   LogLevel = "fatal"
	LogSystem  LogLevel = "system"
	LogError   LogLevel = "error"
	LogCrit    LogLevel = "crit"
	LogWarn    LogLevel = "warn"
	LogInfo    LogLevel = "info"
	LogVerbose LogLevel = "verbose"
	LogDebug   LogLevel = "debug"
)

var LogLevelValues = []string{
	LogFatal, LogSystem, LogError, LogCrit, LogWarn, LogInfo, LogVerbose, LogDebug,
}

// LogFormat is instance.log.format.
type LogFormat = string

const (
	LogFormatPlain LogFormat = "plain"
	LogFormatJSON  LogFormat = "json"
)

var LogFormatValues = []string{LogFormatPlain, LogFormatJSON}

// =============================================================================
// Field names
// =============================================================================

const (
	// root / topology
	FieldTopology          = types.Field("topology")
	FieldClusterName       = types.Field("cluster_name")
	FieldInstanceCount     = types.Field("instance_count")
	FieldReplicationFactor = types.Field("replication_factor")

	// sharding
	FieldSharding    = types.Field("sharding")
	FieldEnabled     = types.Field("enabled")
	FieldBucketCount = types.Field("bucket_count")
	FieldRebalancer  = types.Field("rebalancer")

	// tiers
	FieldTiers     = types.Field("tiers")
	FieldTier      = types.Field("tier")
	FieldName      = types.Field("name")
	FieldCanVote   = types.Field("can_vote")
	FieldCount     = types.Field("count")
	FieldIsSharded = types.Field("sharded")

	// config (picodata.yaml instance.*)
	FieldConfig             = types.Field("config")
	FieldMemtxMemory        = types.Field("memtx_memory")
	FieldMaxTupleSz         = types.Field("memtx_max_tuple_size")
	FieldVinylMemory        = types.Field("vinyl_memory")
	FieldVinylCache         = types.Field("vinyl_cache")
	FieldNetMsgMax          = types.Field("net_msg_max")
	FieldReadahead          = types.Field("readahead")
	FieldCheckpointInterval = types.Field("checkpoint_interval")
	FieldCheckpointCount    = types.Field("checkpoint_count")

	// config — logging
	FieldLogLevel  = types.Field("log_level")
	FieldLogFormat = types.Field("log_format")

	// config — iproto (binary protocol / peer)
	FieldListenPort = types.Field("listen_port")

	// config — pg (PostgreSQL wire protocol)
	FieldPgEnabled = types.Field("pg_enabled")
	FieldPgPort    = types.Field("pg_port")
	FieldPgSSL     = types.Field("pg_ssl")

	// config — http (monitoring / admin)
	FieldHTTPEnabled = types.Field("http_enabled")
	FieldHTTPPort    = types.Field("http_port")

	// config — escape hatch
	FieldExtraParams = types.Field("extra_params")
	FieldParam       = types.Field("param")
	FieldKey         = types.Field("key")
	FieldValue       = types.Field("value")
)
