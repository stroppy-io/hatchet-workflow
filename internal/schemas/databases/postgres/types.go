package postgres

import "github.com/stroppy-io/stroppy-cloud/internal/schemas/types"

// String vocabulary for the PURE PostgreSQL database schema: enum values, field
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
type Topology = string

const (
	TopologySingle          Topology = "single"
	TopologyPrimaryReplicas Topology = "primary_replicas"
	TopologyPatroniHA       Topology = "patroni_ha"
)

var TopologyValues = []string{TopologySingle, TopologyPrimaryReplicas, TopologyPatroniHA}

// PgFlavor is the PostgreSQL distribution.
type PgFlavor = string

const (
	FlavorVanilla     PgFlavor = "vanilla"
	FlavorCitus       PgFlavor = "citus"
	FlavorTimescaleDB PgFlavor = "timescaledb"
)

var PgFlavorValues = []string{FlavorVanilla, FlavorCitus, FlavorTimescaleDB}

// PgMajor is the PostgreSQL major version (int so version gates compare numerically).
type PgMajor = int32

const (
	PgMajor14 PgMajor = 14
	PgMajor15 PgMajor = 15
	PgMajor16 PgMajor = 16
	PgMajor17 PgMajor = 17
)

var PgMajorValues = []int32{PgMajor14, PgMajor15, PgMajor16, PgMajor17}

// DCSType is the Patroni distributed configuration store.
type DCSType = string

const (
	DCSEtcd       DCSType = "etcd"
	DCSConsul     DCSType = "consul"
	DCSZooKeeper  DCSType = "zookeeper"
	DCSKubernetes DCSType = "kubernetes"
)

var DCSTypeValues = []string{DCSEtcd, DCSConsul, DCSZooKeeper, DCSKubernetes}

// ClusterSize is the DCS cluster size (odd for quorum).
type ClusterSize = int32

const (
	ClusterSize1 ClusterSize = 1
	ClusterSize3 ClusterSize = 3
	ClusterSize5 ClusterSize = 5
)

var ClusterSizeValues = []int32{ClusterSize1, ClusterSize3, ClusterSize5}

// SynchronousMode is the Patroni synchronous mode.
type SynchronousMode = string

const (
	SyncModeOff    SynchronousMode = "off"
	SyncModeOn     SynchronousMode = "on"
	SyncModeQuorum SynchronousMode = "quorum"
)

var SynchronousModeValues = []string{SyncModeOff, SyncModeOn, SyncModeQuorum}

// ReplicationMode is async vs synchronous streaming replication.
type ReplicationMode = string

const (
	ReplAsync ReplicationMode = "async"
	ReplSync  ReplicationMode = "sync"
)

var ReplicationModeValues = []string{ReplAsync, ReplSync}

// SynchronousCommit is the postgresql.conf synchronous_commit level.
type SynchronousCommit = string

const (
	SyncCommitOff         SynchronousCommit = "off"
	SyncCommitLocal       SynchronousCommit = "local"
	SyncCommitRemoteWrite SynchronousCommit = "remote_write"
	SyncCommitOn          SynchronousCommit = "on"
	SyncCommitRemoteApply SynchronousCommit = "remote_apply"
)

var SynchronousCommitValues = []string{
	SyncCommitOff, SyncCommitLocal, SyncCommitRemoteWrite, SyncCommitOn, SyncCommitRemoteApply,
}

// SyncMethod is FIRST n (priority) vs ANY n (quorum) synchronous standbys.
type SyncMethod = string

const (
	SyncMethodFirst SyncMethod = "first"
	SyncMethodAny   SyncMethod = "any"
)

var SyncMethodValues = []string{SyncMethodFirst, SyncMethodAny}

// WalLevel is the postgresql.conf wal_level.
type WalLevel = string

const (
	WalLevelReplica WalLevel = "replica"
	WalLevelLogical WalLevel = "logical"
)

var WalLevelValues = []string{WalLevelReplica, WalLevelLogical}

// PoolerType is the connection pooler implementation.
type PoolerType = string

const (
	PoolerPgBouncer PoolerType = "pgbouncer"
	PoolerPgCat     PoolerType = "pgcat"
	PoolerOdyssey   PoolerType = "odyssey"
)

var PoolerTypeValues = []string{PoolerPgBouncer, PoolerPgCat, PoolerOdyssey}

// PoolMode is the pooler pool mode.
type PoolMode = string

const (
	PoolModeSession     PoolMode = "session"
	PoolModeTransaction PoolMode = "transaction"
	PoolModeStatement   PoolMode = "statement"
)

var PoolModeValues = []string{PoolModeSession, PoolModeTransaction, PoolModeStatement}

// PoolerAuthType is the pooler auth method.
type PoolerAuthType = string

const (
	PoolerAuthTrust PoolerAuthType = "trust"
	PoolerAuthMD5   PoolerAuthType = "md5"
	PoolerAuthSCRAM PoolerAuthType = "scram-sha-256"
	PoolerAuthHBA   PoolerAuthType = "hba"
	PoolerAuthCert  PoolerAuthType = "cert"
)

var PoolerAuthTypeValues = []string{
	PoolerAuthTrust, PoolerAuthMD5, PoolerAuthSCRAM, PoolerAuthHBA, PoolerAuthCert,
}

// WorkloadConnectTarget is which logical endpoint the benchmark connects to (intent).
type WorkloadConnectTarget = string

const (
	ConnectPrimary  WorkloadConnectTarget = "primary"
	ConnectPooler   WorkloadConnectTarget = "pooler"
	ConnectReadPort WorkloadConnectTarget = "read_port"
)

var WorkloadConnectTargetValues = []string{ConnectPrimary, ConnectPooler, ConnectReadPort}

// HugePages is the postgresql.conf huge_pages setting.
type HugePages = string

const (
	HugePagesTry HugePages = "try"
	HugePagesOn  HugePages = "on"
	HugePagesOff HugePages = "off"
)

var HugePagesValues = []string{HugePagesTry, HugePagesOn, HugePagesOff}

// WalCompression is the postgresql.conf wal_compression setting. lz4/zstd need PG15+.
type WalCompression = string

const (
	WalCompressionOff  WalCompression = "off"
	WalCompressionPglz WalCompression = "pglz"
	WalCompressionLz4  WalCompression = "lz4"
	WalCompressionZstd WalCompression = "zstd"
)

var WalCompressionValues = []string{WalCompressionOff, WalCompressionPglz, WalCompressionLz4, WalCompressionZstd}

// ArchiveMode is the postgresql.conf archive_mode setting.
type ArchiveMode = string

const (
	ArchiveOff    ArchiveMode = "off"
	ArchiveOn     ArchiveMode = "on"
	ArchiveAlways ArchiveMode = "always"
)

var ArchiveModeValues = []string{ArchiveOff, ArchiveOn, ArchiveAlways}

// TrackFunctions is the postgresql.conf track_functions setting.
type TrackFunctions = string

const (
	TrackFuncNone TrackFunctions = "none"
	TrackFuncPl   TrackFunctions = "pl"
	TrackFuncAll  TrackFunctions = "all"
)

var TrackFunctionsValues = []string{TrackFuncNone, TrackFuncPl, TrackFuncAll}

// ComputeQueryID is the postgresql.conf compute_query_id setting (PG14+).
type ComputeQueryID = string

const (
	ComputeQueryIDOff     ComputeQueryID = "off"
	ComputeQueryIDOn      ComputeQueryID = "on"
	ComputeQueryIDAuto    ComputeQueryID = "auto"
	ComputeQueryIDRegress ComputeQueryID = "regress"
)

var ComputeQueryIDValues = []string{
	ComputeQueryIDOff, ComputeQueryIDOn, ComputeQueryIDAuto, ComputeQueryIDRegress,
}

// AuthMethod is a pg_hba auth method (also used for the cluster default).
type AuthMethod = string

const (
	AuthTrust  AuthMethod = "trust"
	AuthSCRAM  AuthMethod = "scram-sha-256"
	AuthMD5    AuthMethod = "md5"
	AuthCert   AuthMethod = "cert"
	AuthPeer   AuthMethod = "peer"
	AuthReject AuthMethod = "reject"
)

var (
	AuthMethodValues = []string{AuthTrust, AuthSCRAM, AuthMD5, AuthCert, AuthPeer}
	HBAMethodValues  = []string{AuthTrust, AuthSCRAM, AuthMD5, AuthCert, AuthReject}
)

// PasswordEncryption is the password_encryption family.
type PasswordEncryption = string

const (
	PwEncSCRAM PasswordEncryption = "scram-sha-256"
	PwEncMD5   PasswordEncryption = "md5"
)

var PasswordEncryptionValues = []string{PwEncSCRAM, PwEncMD5}

// HBAType is a pg_hba connection type.
type HBAType = string

const (
	HBALocal     HBAType = "local"
	HBAHost      HBAType = "host"
	HBAHostSSL   HBAType = "hostssl"
	HBAHostNoSSL HBAType = "hostnossl"
)

var HBATypeValues = []string{HBALocal, HBAHost, HBAHostSSL, HBAHostNoSSL}

// ExtensionName is an installable PostgreSQL extension.
type ExtensionName = string

const (
	ExtPgStatStatements ExtensionName = "pg_stat_statements"
	ExtPgPrewarm        ExtensionName = "pg_prewarm"
	ExtAutoExplain      ExtensionName = "auto_explain"
	ExtPgBufferCache    ExtensionName = "pg_buffercache"
	ExtPgStatTuple      ExtensionName = "pgstattuple"
	ExtPageInspect      ExtensionName = "pageinspect"
	ExtCitus            ExtensionName = "citus"
	ExtTimescaleDB      ExtensionName = "timescaledb"
	ExtPostgresFDW      ExtensionName = "postgres_fdw"
	ExtPgPartman        ExtensionName = "pg_partman"
)

var ExtensionNameValues = []string{
	ExtPgStatStatements, ExtPgPrewarm, ExtAutoExplain, ExtPgBufferCache, ExtPgStatTuple,
	ExtPageInspect, ExtCitus, ExtTimescaleDB, ExtPostgresFDW, ExtPgPartman,
}

// =============================================================================
// Field names
// =============================================================================

const (
	// root
	FieldTopology     = types.Field("topology")
	FieldPgMajor      = types.Field("pg_major")
	FieldPgFlavor     = types.Field("pg_flavor")
	FieldReplicaCount = types.Field("replica_count")

	// ha
	FieldHA                   = types.Field("ha")
	FieldDCS                  = types.Field("dcs")
	FieldType                 = types.Field("type")
	FieldClusterSize          = types.Field("cluster_size")
	FieldPatroni              = types.Field("patroni")
	FieldTTL                  = types.Field("ttl")
	FieldLoopWait             = types.Field("loop_wait")
	FieldRetryTimeout         = types.Field("retry_timeout")
	FieldMaxLagOnFailover     = types.Field("maximum_lag_on_failover")
	FieldMasterStartTimeout   = types.Field("master_start_timeout")
	FieldSynchronousMode      = types.Field("synchronous_mode")
	FieldSynchronousNodeCount = types.Field("synchronous_node_count")

	// replication
	FieldReplication         = types.Field("replication")
	FieldMode                = types.Field("mode")
	FieldSynchronousCommit   = types.Field("synchronous_commit")
	FieldSyncMethod          = types.Field("sync_method")
	FieldWalLevel            = types.Field("wal_level")
	FieldMaxWalSenders       = types.Field("max_wal_senders")
	FieldMaxReplicationSlots = types.Field("max_replication_slots")
	FieldUseReplicationSlots = types.Field("use_replication_slots")
	FieldHotStandbyFeedback  = types.Field("hot_standby_feedback")
	FieldCascading           = types.Field("cascading")
	FieldWalKeepSize         = types.Field("wal_keep_size")

	// pooling
	FieldPooling         = types.Field("pooling")
	FieldUsePooler       = types.Field("use_pooler")
	FieldPooler          = types.Field("pooler")
	FieldPoolMode        = types.Field("pool_mode")
	FieldMaxClientConn   = types.Field("max_client_conn")
	FieldDefaultPoolSize = types.Field("default_pool_size")
	FieldAuthType        = types.Field("auth_type")

	// routing intent (no ports/health/LB-impl — those are deployment, live at cluster)
	FieldRouting               = types.Field("routing")
	FieldReadWriteSplit        = types.Field("read_write_split")
	FieldWorkloadConnectTarget = types.Field("workload_connect_target")

	// tuning — Memory
	FieldTuning             = types.Field("tuning")
	FieldSharedBuffers      = types.Field("shared_buffers")
	FieldWorkMem            = types.Field("work_mem")
	FieldMaintenanceWorkMem = types.Field("maintenance_work_mem")
	FieldEffectiveCacheSize = types.Field("effective_cache_size")
	FieldWalBuffers         = types.Field("wal_buffers")
	FieldTempBuffers        = types.Field("temp_buffers")
	FieldHashMemMultiplier  = types.Field("hash_mem_multiplier")
	FieldHugePages          = types.Field("huge_pages")

	// tuning — WAL & durability
	FieldMaxWalSize     = types.Field("max_wal_size")
	FieldMinWalSize     = types.Field("min_wal_size")
	FieldWalCompression = types.Field("wal_compression")
	FieldFullPageWrites = types.Field("full_page_writes")
	FieldFsync          = types.Field("fsync")
	FieldWalWriterDelay = types.Field("wal_writer_delay")
	FieldWalLogHints    = types.Field("wal_log_hints")
	FieldCommitDelay    = types.Field("commit_delay")
	FieldCommitSiblings = types.Field("commit_siblings")
	FieldArchiveMode    = types.Field("archive_mode")
	FieldArchiveCommand = types.Field("archive_command")

	// tuning — Checkpoints & bgwriter
	FieldCheckpointCompletionTarget = types.Field("checkpoint_completion_target")
	FieldCheckpointTimeout          = types.Field("checkpoint_timeout")
	FieldCheckpointFlushAfter       = types.Field("checkpoint_flush_after")
	FieldBgwriterDelay              = types.Field("bgwriter_delay")
	FieldBgwriterLruMaxpages        = types.Field("bgwriter_lru_maxpages")
	FieldBgwriterLruMultiplier      = types.Field("bgwriter_lru_multiplier")

	// tuning — Parallelism
	FieldMaxConnections               = types.Field("max_connections")
	FieldMaxWorkerProcesses           = types.Field("max_worker_processes")
	FieldMaxParallelWorkers           = types.Field("max_parallel_workers")
	FieldMaxParallelWorkersPerGather  = types.Field("max_parallel_workers_per_gather")
	FieldMaxParallelMaintenanceWorker = types.Field("max_parallel_maintenance_workers")

	// tuning — Planner & I/O
	FieldRandomPageCost          = types.Field("random_page_cost")
	FieldSeqPageCost             = types.Field("seq_page_cost")
	FieldEffectiveIoConcurrency  = types.Field("effective_io_concurrency")
	FieldMaintenanceIoConcurrenc = types.Field("maintenance_io_concurrency")
	FieldDefaultStatisticsTarget = types.Field("default_statistics_target")
	FieldJit                     = types.Field("jit")
	FieldJitAboveCost            = types.Field("jit_above_cost")

	// tuning — Autovacuum
	FieldAutovacuum                   = types.Field("autovacuum")
	FieldAutovacuumMaxWorkers         = types.Field("autovacuum_max_workers")
	FieldAutovacuumNaptime            = types.Field("autovacuum_naptime")
	FieldAutovacuumVacuumScaleFactor  = types.Field("autovacuum_vacuum_scale_factor")
	FieldAutovacuumAnalyzeScaleFactor = types.Field("autovacuum_analyze_scale_factor")
	FieldAutovacuumVacuumCostLimit    = types.Field("autovacuum_vacuum_cost_limit")
	FieldAutovacuumVacuumCostDelay    = types.Field("autovacuum_vacuum_cost_delay")

	// tuning — Logging / observability
	FieldLogMinDurationStatement = types.Field("log_min_duration_statement")
	FieldLogCheckpoints          = types.Field("log_checkpoints")
	FieldTrackIoTiming           = types.Field("track_io_timing")
	FieldTrackFunctions          = types.Field("track_functions")
	FieldTrackActivityQuerySize  = types.Field("track_activity_query_size")
	FieldComputeQueryID          = types.Field("compute_query_id")

	// tuning — escape hatch
	FieldExtraParams = types.Field("extra_params")
	FieldParam       = types.Field("param")
	FieldKey         = types.Field("key")
	FieldValue       = types.Field("value")

	// auth
	FieldAuth               = types.Field("auth")
	FieldAuthMethod         = types.Field("auth_method")
	FieldPasswordEncryption = types.Field("password_encryption")
	FieldSSL                = types.Field("ssl")
	FieldListenAddresses    = types.Field("listen_addresses")
	FieldHBARules           = types.Field("hba_rules")
	FieldRule               = types.Field("rule")
	FieldDatabase           = types.Field("database")
	FieldUser               = types.Field("user")
	FieldCIDR               = types.Field("cidr")
	FieldMethod             = types.Field("method")

	// extensions
	FieldExtensions = types.Field("extensions")
	FieldEnabled    = types.Field("enabled")
	FieldExt        = types.Field("ext")
	FieldName       = types.Field("name")
)
