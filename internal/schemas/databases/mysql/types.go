package mysql

import "github.com/stroppy-io/stroppy-cloud/internal/schemas/types"

// String vocabulary for the PURE MySQL database schema: enum values, field names,
// and the expr paths used by its (DB-internal) gates. This schema is
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
	TopologySingle           Topology = "single"
	TopologyPrimaryReplicas  Topology = "primary_replicas"
	TopologyGroupReplication Topology = "group_replication"
)

var TopologyValues = []string{TopologySingle, TopologyPrimaryReplicas, TopologyGroupReplication}

// MySQLVersion is the MySQL server version (string so 8.0/8.4 keep their dotted form,
// which is the token used by the apt repo component downstream).
type MySQLVersion = string

const (
	Version80 MySQLVersion = "8.0"
	Version84 MySQLVersion = "8.4"
)

var MySQLVersionValues = []string{Version80, Version84}

// ReplicationMode is the XOR replication strategy. The cross-field constraint
// (mode must match topology) lives at the cluster layer — NOT here.
type ReplicationMode = string

const (
	ReplAsync    ReplicationMode = "async"
	ReplSemiSync ReplicationMode = "semi_sync"
	ReplGroup    ReplicationMode = "group_replication"
)

var ReplicationModeValues = []string{ReplAsync, ReplSemiSync, ReplGroup}

// GtidMode is the gtid_mode setting.
type GtidMode = string

const (
	GtidOn          GtidMode = "ON"
	GtidOff         GtidMode = "OFF"
	GtidOnPermisive GtidMode = "ON_PERMISSIVE"
)

var GtidModeValues = []string{GtidOn, GtidOff, GtidOnPermisive}

// BinlogFormat is the binlog_format. ROW is the MySQL 8.x default and the only
// format valid for Group Replication.
type BinlogFormat = string

const (
	BinlogRow       BinlogFormat = "ROW"
	BinlogStatement BinlogFormat = "STATEMENT"
	BinlogMixed     BinlogFormat = "MIXED"
)

var BinlogFormatValues = []string{BinlogRow, BinlogStatement, BinlogMixed}

// FlushMethod is the innodb_flush_method.
type FlushMethod = string

const (
	FlushODirect        FlushMethod = "O_DIRECT"
	FlushODirectNoFsync FlushMethod = "O_DIRECT_NO_FSYNC"
	FlushFsync          FlushMethod = "fsync"
)

var FlushMethodValues = []string{FlushODirect, FlushODirectNoFsync, FlushFsync}

// GroupReplMode is single-primary vs multi-primary InnoDB Group Replication.
type GroupReplMode = string

const (
	GRSinglePrimary GroupReplMode = "single_primary"
	GRMultiPrimary  GroupReplMode = "multi_primary"
)

var GroupReplModeValues = []string{GRSinglePrimary, GRMultiPrimary}

// GroupReplConsistency is the group_replication_consistency level.
type GroupReplConsistency = string

const (
	GRConsistencyEventual        GroupReplConsistency = "EVENTUAL"
	GRConsistencyBeforeOnPrimary GroupReplConsistency = "BEFORE_ON_PRIMARY_FAILOVER"
	GRConsistencyBefore          GroupReplConsistency = "BEFORE"
	GRConsistencyAfter           GroupReplConsistency = "AFTER"
	GRConsistencyBeforeAndAfter  GroupReplConsistency = "BEFORE_AND_AFTER"
)

var GroupReplConsistencyValues = []string{
	GRConsistencyEventual, GRConsistencyBeforeOnPrimary, GRConsistencyBefore,
	GRConsistencyAfter, GRConsistencyBeforeAndAfter,
}

// AuthPlugin is the default_authentication_plugin / authentication_policy family.
// caching_sha2_password is the MySQL 8.0+/8.4 default.
type AuthPlugin = string

const (
	AuthCachingSHA2 AuthPlugin = "caching_sha2_password"
	AuthNative      AuthPlugin = "mysql_native_password"
)

var AuthPluginValues = []string{AuthCachingSHA2, AuthNative}

// SSLMode is the require_secure_transport intent expressed as a TLS posture.
type SSLMode = string

const (
	SSLDisabled  SSLMode = "disabled"
	SSLPreferred SSLMode = "preferred"
	SSLRequired  SSLMode = "required"
)

var SSLModeValues = []string{SSLDisabled, SSLPreferred, SSLRequired}

// ProxySQLConnMux toggles connection multiplexing intent on the proxy.
type ProxyHostgroupRole = string

const (
	HostgroupWriter ProxyHostgroupRole = "writer"
	HostgroupReader ProxyHostgroupRole = "reader"
)

var ProxyHostgroupRoleValues = []string{HostgroupWriter, HostgroupReader}

// WorkloadConnectTarget is which logical endpoint the benchmark connects to (intent).
type WorkloadConnectTarget = string

const (
	ConnectPrimary WorkloadConnectTarget = "primary"
	ConnectProxy   WorkloadConnectTarget = "proxy"
	ConnectReader  WorkloadConnectTarget = "reader"
)

var WorkloadConnectTargetValues = []string{ConnectPrimary, ConnectProxy, ConnectReader}

// =============================================================================
// Field names
// =============================================================================

const (
	// root
	FieldTopology     = types.Field("topology")
	FieldVersion      = types.Field("version")
	FieldReplicaCount = types.Field("replica_count")

	// replication
	FieldReplication              = types.Field("replication")
	FieldMode                     = types.Field("mode")
	FieldGtidMode                 = types.Field("gtid_mode")
	FieldEnforceGtidConsistency   = types.Field("enforce_gtid_consistency")
	FieldBinlogFormat             = types.Field("binlog_format")
	FieldLogBin                   = types.Field("log_bin")
	FieldLogReplicaUpdates        = types.Field("log_replica_updates")
	FieldReplicaParallelWorkers   = types.Field("replica_parallel_workers")
	FieldReplicaPreserveCommitOrd = types.Field("replica_preserve_commit_order")
	FieldSemiSyncTimeout          = types.Field("semi_sync_timeout")
	FieldSemiSyncWaitForReplica   = types.Field("semi_sync_wait_for_replica_count")

	// group replication (sub-object of replication)
	FieldGroupReplication      = types.Field("group_replication")
	FieldGroupName             = types.Field("group_name")
	FieldGroupReplMode         = types.Field("group_mode")
	FieldGroupConsistency      = types.Field("consistency")
	FieldGroupLocalAddressPort = types.Field("local_address_port")
	FieldGroupAutoRejoinTries  = types.Field("auto_rejoin_tries")
	FieldGroupExitStateAction  = types.Field("exit_state_action")
	FieldGroupFlowControlMode  = types.Field("flow_control_mode")

	// proxy intent (ProxySQL) — config only, no machine
	FieldProxy          = types.Field("proxy")
	FieldUseProxy       = types.Field("use_proxy")
	FieldProxySQL       = types.Field("proxysql")
	FieldMonitorUser    = types.Field("monitor_user")
	FieldMaxConnections = types.Field("max_connections")
	FieldHostgroups     = types.Field("hostgroups")
	FieldHostgroup      = types.Field("hostgroup")
	FieldHostgroupID    = types.Field("id")
	FieldHostgroupRole  = types.Field("role")
	FieldQueryRulesFast = types.Field("fast_routing")

	// tuning — InnoDB
	FieldTuning                  = types.Field("tuning")
	FieldInnodbBufferPoolSize    = types.Field("innodb_buffer_pool_size")
	FieldInnodbBufferPoolInst    = types.Field("innodb_buffer_pool_instances")
	FieldInnodbRedoLogCapacity   = types.Field("innodb_redo_log_capacity")
	FieldInnodbLogFileSize       = types.Field("innodb_log_file_size")
	FieldInnodbFlushMethod       = types.Field("innodb_flush_method")
	FieldInnodbFlushLogAtTrxComm = types.Field("innodb_flush_log_at_trx_commit")
	FieldInnodbIoCapacity        = types.Field("innodb_io_capacity")
	FieldInnodbIoCapacityMax     = types.Field("innodb_io_capacity_max")
	FieldInnodbReadIoThreads     = types.Field("innodb_read_io_threads")
	FieldInnodbWriteIoThreads    = types.Field("innodb_write_io_threads")
	FieldInnodbAutoincLockMode   = types.Field("innodb_autoinc_lock_mode")
	FieldInnodbDedicatedServer   = types.Field("innodb_dedicated_server")
	FieldInnodbDoublewrite       = types.Field("innodb_doublewrite")

	// tuning — Connections
	FieldMaxConns         = types.Field("max_connections")
	FieldThreadCacheSize  = types.Field("thread_cache_size")
	FieldTableOpenCache   = types.Field("table_open_cache")
	FieldTableDefCache    = types.Field("table_definition_cache")
	FieldThreadStack      = types.Field("thread_stack")
	FieldMaxAllowedPacket = types.Field("max_allowed_packet")
	FieldTmpTableSize     = types.Field("tmp_table_size")

	// tuning — Durability / binlog
	FieldSyncBinlog          = types.Field("sync_binlog")
	FieldBinlogExpireSeconds = types.Field("binlog_expire_logs_seconds")
	FieldBinlogRowImage      = types.Field("binlog_row_image")
	FieldBinlogCacheSize     = types.Field("binlog_cache_size")
	FieldMaxBinlogSize       = types.Field("max_binlog_size")

	// tuning — escape hatch
	FieldExtraParams = types.Field("extra_params")
	FieldParam       = types.Field("param")
	FieldKey         = types.Field("key")
	FieldValue       = types.Field("value")

	// auth
	FieldAuth                = types.Field("auth")
	FieldDefaultAuthPlugin   = types.Field("default_authentication_plugin")
	FieldSSLMode             = types.Field("ssl_mode")
	FieldBindAddress         = types.Field("bind_address")
	FieldRequireSecureTransp = types.Field("require_secure_transport")
	FieldUsers               = types.Field("users")
	FieldUser                = types.Field("user")
	FieldName                = types.Field("name")
	FieldHost                = types.Field("host")
	FieldPlugin              = types.Field("plugin")

	// routing intent
	FieldRouting               = types.Field("routing")
	FieldReadWriteSplit        = types.Field("read_write_split")
	FieldWorkloadConnectTarget = types.Field("workload_connect_target")
)
