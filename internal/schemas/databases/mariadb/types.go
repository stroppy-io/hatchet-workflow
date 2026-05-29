package mariadb

import "github.com/stroppy-io/stroppy-cloud/internal/schemas/types"

// String vocabulary for the PURE MariaDB database schema: enum values, field
// names, and the expr paths used by its (DB-internal) gates. This schema is
// provider-agnostic — it knows nothing about machines, disks, providers or
// placement. Those live in the provider schema and the cluster (top) schema.
//
// Enum value types are string aliases so they pass straight into
// utils.StrEnum(...).In() / .Default(); each ships a <Name>Values slice.
//
// MariaDB is kept aligned with the MySQL schema where the my.cnf surface is
// shared (InnoDB, connections, binlog) and diverges where it is real: galera
// (instead of InnoDB Group Replication), wsrep_* knobs, and no caching_sha2
// default authentication plugin.

// =============================================================================
// Enum value types (string) + values
// =============================================================================

// Topology is the cluster shape — the root discriminator. MariaDB has no InnoDB
// Group Replication; its multi-primary clustering is Galera (wsrep).
type Topology = string

const (
	TopologySingle          Topology = "single"
	TopologyPrimaryReplicas Topology = "primary_replicas"
	TopologyGalera          Topology = "galera"
)

var TopologyValues = []string{TopologySingle, TopologyPrimaryReplicas, TopologyGalera}

// MariaDBVersion is the MariaDB server version (string so 10.11/11.4 keep their
// dotted form, which is the token passed to mariadb_repo_setup --version downstream).
type MariaDBVersion = string

const (
	Version1011 MariaDBVersion = "10.11"
	Version114  MariaDBVersion = "11.4"
)

var MariaDBVersionValues = []string{Version1011, Version114}

// ReplicationMode is the XOR replication strategy. The cross-field constraint
// (mode must match topology) lives at the cluster layer — NOT here.
type ReplicationMode = string

const (
	ReplAsync    ReplicationMode = "async"
	ReplSemiSync ReplicationMode = "semi_sync"
	ReplGalera   ReplicationMode = "galera"
)

var ReplicationModeValues = []string{ReplAsync, ReplSemiSync, ReplGalera}

// BinlogFormat is the binlog_format. ROW is required for Galera.
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

// WsrepSstMethod is the Galera State Snapshot Transfer method.
type WsrepSstMethod = string

const (
	SstMariabackup WsrepSstMethod = "mariabackup"
	SstRsync       WsrepSstMethod = "rsync"
	SstMysqldump   WsrepSstMethod = "mysqldump"
)

var WsrepSstMethodValues = []string{SstMariabackup, SstRsync, SstMysqldump}

// AuthPlugin is the authentication plugin family. MariaDB has NO caching_sha2
// default; the historical default is mysql_native_password, with ed25519 as the
// modern strong option.
type AuthPlugin = string

const (
	AuthNative  AuthPlugin = "mysql_native_password"
	AuthEd25519 AuthPlugin = "ed25519"
)

var AuthPluginValues = []string{AuthNative, AuthEd25519}

// SSLMode is the require_secure_transport intent expressed as a TLS posture.
type SSLMode = string

const (
	SSLDisabled  SSLMode = "disabled"
	SSLPreferred SSLMode = "preferred"
	SSLRequired  SSLMode = "required"
)

var SSLModeValues = []string{SSLDisabled, SSLPreferred, SSLRequired}

// ProxyHostgroupRole is the ProxySQL hostgroup role.
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
	FieldReplication            = types.Field("replication")
	FieldMode                   = types.Field("mode")
	FieldGtidStrictMode         = types.Field("gtid_strict_mode")
	FieldBinlogFormat           = types.Field("binlog_format")
	FieldLogBin                 = types.Field("log_bin")
	FieldLogReplicaUpdates      = types.Field("log_slave_updates")
	FieldReplicaParallelThreads = types.Field("slave_parallel_threads")
	FieldReplicaParallelMode    = types.Field("slave_parallel_mode")
	FieldSemiSyncTimeout        = types.Field("rpl_semi_sync_master_timeout")
	FieldSemiSyncWaitNoReplica  = types.Field("rpl_semi_sync_master_wait_no_slave")

	// galera (sub-object of replication)
	FieldGalera           = types.Field("galera")
	FieldClusterName      = types.Field("wsrep_cluster_name")
	FieldSstMethod        = types.Field("wsrep_sst_method")
	FieldProviderOptions  = types.Field("wsrep_provider_options")
	FieldSlaveThreads     = types.Field("wsrep_slave_threads")
	FieldGcacheSize       = types.Field("gcache_size")
	FieldLocalAddressPort = types.Field("local_address_port")
	FieldGtidDomainID     = types.Field("gtid_domain_id")

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
	FieldInnodbLogFileSize       = types.Field("innodb_log_file_size")
	FieldInnodbLogFiles          = types.Field("innodb_log_files_in_group")
	FieldInnodbFlushMethod       = types.Field("innodb_flush_method")
	FieldInnodbFlushLogAtTrxComm = types.Field("innodb_flush_log_at_trx_commit")
	FieldInnodbIoCapacity        = types.Field("innodb_io_capacity")
	FieldInnodbIoCapacityMax     = types.Field("innodb_io_capacity_max")
	FieldInnodbReadIoThreads     = types.Field("innodb_read_io_threads")
	FieldInnodbWriteIoThreads    = types.Field("innodb_write_io_threads")
	FieldInnodbAutoincLockMode   = types.Field("innodb_autoinc_lock_mode")
	FieldInnodbDoublewrite       = types.Field("innodb_doublewrite")

	// tuning — Connections
	FieldMaxConns         = types.Field("max_connections")
	FieldThreadCacheSize  = types.Field("thread_cache_size")
	FieldThreadPoolSize   = types.Field("thread_pool_size")
	FieldTableOpenCache   = types.Field("table_open_cache")
	FieldTableDefCache    = types.Field("table_definition_cache")
	FieldThreadStack      = types.Field("thread_stack")
	FieldMaxAllowedPacket = types.Field("max_allowed_packet")
	FieldTmpTableSize     = types.Field("tmp_table_size")

	// tuning — Durability / binlog
	FieldSyncBinlog          = types.Field("sync_binlog")
	FieldBinlogExpireSeconds = types.Field("expire_logs_days")
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
