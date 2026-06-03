package types

// Provider is the infrastructure provider for machine provisioning.
type Provider string

const (
	ProviderYandex Provider = "yandex"
	ProviderDocker Provider = "docker" // local emulation via apt/rpm in containers
)

// DatabaseKind is the database to deploy and test.
type DatabaseKind string

const (
	DatabasePostgres   DatabaseKind = "postgres"
	DatabaseMySQL      DatabaseKind = "mysql"
	DatabaseMariaDB    DatabaseKind = "mariadb"
	DatabasePicodata   DatabaseKind = "picodata"
	DatabaseYDB        DatabaseKind = "ydb"
	DatabaseYDBManaged DatabaseKind = "ydb-managed" // Yandex Cloud Managed YDB (serverless or dedicated)
	DatabaseCockroach  DatabaseKind = "cockroach"
	DatabaseTesting    DatabaseKind = "testing" // Synthetic targets: stroppy noop driver or pg-noop blackhole
)

// Phase is the DAG node type identifier for each run stage.
type Phase string

const (
	PhaseNetwork            Phase = "network"           // subnet/VPC allocation
	PhaseMachines           Phase = "machines"          // VM provisioning or Docker container creation
	PhaseInstallDB          Phase = "install_db"        // database binary installation
	PhaseConfigureDB        Phase = "configure_db"      // database cluster configuration
	PhaseInstallMonitor     Phase = "install_monitor"   // monitoring stack deployment
	PhaseConfigureMonitor   Phase = "configure_monitor" // monitoring configuration & targets
	PhaseInstallStroppy     Phase = "install_stroppy"   // stroppy deployment on a dedicated machine
	PhaseRunStroppy         Phase = "run_stroppy"       // stroppy test execution
	PhaseInstallEtcd        Phase = "install_etcd"
	PhaseConfigureEtcd      Phase = "configure_etcd"
	PhaseInstallProxy       Phase = "install_proxy" // HAProxy/ProxySQL
	PhaseConfigureProxy     Phase = "configure_proxy"
	PhaseInstallPgBouncer   Phase = "install_pgbouncer"
	PhaseConfigurePgBouncer Phase = "configure_pgbouncer"
	PhaseInstallPatroni     Phase = "install_patroni"
	PhaseConfigurePatroni   Phase = "configure_patroni"
	PhaseInitYDBCluster     Phase = "init_ydb_cluster"
	PhaseStartYDBDatabase   Phase = "start_ydb_database"
	PhaseInitCockroach      Phase = "init_cockroach" // one-shot `cockroach init` after every node is up
	PhaseTeardown           Phase = "teardown"       // infrastructure cleanup
)

// MachineRole distinguishes machines by purpose within a run.
type MachineRole string

const (
	RoleDatabase    MachineRole = "database"
	RoleMonitor     MachineRole = "monitor"
	RoleStroppy     MachineRole = "stroppy"
	RoleEtcd        MachineRole = "etcd"
	RoleProxy       MachineRole = "proxy" // HAProxy / ProxySQL / LB
	RolePgBouncer   MachineRole = "pgbouncer"
	RoleYDBStorage  MachineRole = "ydb-storage"  // static (storage) YDB node
	RoleYDBDatabase MachineRole = "ydb-database" // dynamic (compute) YDB node
)

// MachineSpec describes a single machine to provision.
type MachineSpec struct {
	Role     MachineRole `json:"role"`
	Count    int         `json:"count"`
	CPUs     int         `json:"cpus"`
	MemoryMB int         `json:"memory_mb"`
	DiskGB   int         `json:"disk_gb"`
	DiskType string      `json:"disk_type,omitempty"`
	// SecondaryDisks are extra block devices attached raw alongside the boot
	// disk. Today only consumed by YDB (storage nodes point pdisk at the raw
	// device when one is present). Other engines ignore the field.
	SecondaryDisks []SecondaryDisk `json:"secondary_disks,omitempty"`
	// Placement controls cloud-zone placement for this machine group. When
	// omitted, the provider's default single-zone placement is used.
	Placement *PlacementSpec `json:"placement,omitempty"`
}

// SecondaryDisk is a raw block device attached to a VM in addition to its
// boot disk. The agent identifies it by /dev/disk/by-id/virtio-<DeviceName>
// inside the guest, so DeviceName must be unique per VM.
type SecondaryDisk struct {
	DeviceName string `json:"device_name"` // stable name, surfaces as virtio-<name> in the guest
	SizeGB     int    `json:"size_gb"`
	Type       string `json:"type,omitempty"` // network-ssd / network-ssd-nonreplicated / etc; defaults to network-ssd
}

// PlacementSpec controls how instances in a MachineSpec are spread across
// cloud zones. "round-robin" distributes instances over Zones by index;
// "single" keeps every instance in the first zone.
type PlacementSpec struct {
	Strategy string   `json:"strategy,omitempty"` // "single" | "round-robin"
	Zones    []string `json:"zones,omitempty"`    // provider-native zone names; empty means provider default/derived zones
}

// --- Database topologies ---

// PostgresTopology describes a Postgres cluster layout.
// Each component has its own Options map for configuration tuning.
type PostgresTopology struct {
	Master       MachineSpec   `json:"master"`
	Replicas     []MachineSpec `json:"replicas,omitempty"`
	HAProxy      *MachineSpec  `json:"haproxy,omitempty"`
	PgBouncer    bool          `json:"pgbouncer"` // colocated on each PG node
	Patroni      bool          `json:"patroni"`
	Etcd         bool          `json:"etcd"` // colocated on PG nodes (up to 3)
	SyncReplicas int           `json:"sync_replicas"`
	// Per-component configuration options.
	MasterOptions    map[string]string `json:"master_options,omitempty"`    // postgresql.conf for master
	ReplicaOptions   map[string]string `json:"replica_options,omitempty"`   // postgresql.conf for replicas
	HAProxyOptions   map[string]string `json:"haproxy_options,omitempty"`   // haproxy.cfg tuning
	PgBouncerOptions map[string]string `json:"pgbouncer_options,omitempty"` // pgbouncer.ini tuning
	PatroniOptions   map[string]string `json:"patroni_options,omitempty"`   // patroni.yml tuning
	EtcdOptions      map[string]string `json:"etcd_options,omitempty"`      // etcd tuning
}

// MySQLTopology describes a MySQL cluster layout.
type MySQLTopology struct {
	Primary   MachineSpec   `json:"primary"`
	Replicas  []MachineSpec `json:"replicas,omitempty"`
	ProxySQL  *MachineSpec  `json:"proxysql,omitempty"` // dedicated ProxySQL node(s)
	GroupRepl bool          `json:"group_replication"`
	SemiSync  bool          `json:"semi_sync"` // semi-synchronous replication (when no GR)
	// Per-component configuration options.
	PrimaryOptions  map[string]string `json:"primary_options,omitempty"`  // my.cnf for primary
	ReplicaOptions  map[string]string `json:"replica_options,omitempty"`  // my.cnf for replicas
	ProxySQLOptions map[string]string `json:"proxysql_options,omitempty"` // proxysql.cnf tuning
}

// PicodataTopology describes a Picodata cluster layout.
type PicodataTopology struct {
	Instances   []MachineSpec  `json:"instances"`
	HAProxy     *MachineSpec   `json:"haproxy,omitempty"` // LB for pgproto
	Replication int            `json:"replication_factor"`
	Shards      int            `json:"shards"`
	Tiers       []PicodataTier `json:"tiers,omitempty"` // for scale deployments
	// Per-component configuration options.
	InstanceOptions map[string]string `json:"instance_options,omitempty"` // picodata.yaml tuning
	HAProxyOptions  map[string]string `json:"haproxy_options,omitempty"`  // haproxy.cfg tuning
}

// CockroachTopology describes a CockroachDB cluster — homogeneous N-node
// deployment, no master/replica split. Each node runs the same `cockroach`
// binary with --join flags pointing at the others; one node runs the
// one-shot `cockroach init` to bootstrap. CockroachDB takes most config
// via CLI flags rather than a config file, so Options here applies as
// `SET CLUSTER SETTING <key> = <value>` post-init (or as additional
// startup flags if prefixed with "flag:").
type CockroachTopology struct {
	Nodes   MachineSpec       `json:"nodes"`
	Options map[string]string `json:"options,omitempty"`
}

// TestingMode selects which synthetic target a Testing preset runs.
type TestingMode string

const (
	// TestingNoopDriver runs only the stroppy runner and uses stroppy's noop driver.
	TestingNoopDriver TestingMode = "noop-driver"
	// TestingPgNoop runs pg-noop, a PostgreSQL-compatible blackhole endpoint.
	TestingPgNoop TestingMode = "pg-noop"
)

// PgNoopConfig configures the pg-noop blackhole server.
type PgNoopConfig struct {
	Version string `json:"version,omitempty"`
	Port    int    `json:"port,omitempty"`
	Workers int    `json:"workers,omitempty"`
}

// TestingTopology describes synthetic test targets used for tooling checks.
// noop-driver provisions no database machine; pg-noop provisions one database
// VM/container that accepts PostgreSQL wire-protocol traffic and discards it.
type TestingTopology struct {
	Mode     TestingMode   `json:"mode"`
	Database *MachineSpec  `json:"database,omitempty"`
	PgNoop   *PgNoopConfig `json:"pg_noop,omitempty"`
}

// YDBTopology describes a YDB cluster layout.
type YDBTopology struct {
	Storage           MachineSpec       `json:"storage"`                       // static (storage) nodes
	Database          *MachineSpec      `json:"database,omitempty"`            // dynamic (compute) nodes; nil = combined mode
	HAProxy           *MachineSpec      `json:"haproxy,omitempty"`             // optional load balancer
	FaultTolerance    string            `json:"fault_tolerance"`               // "none", "block-4-2", "mirror-3-dc"
	FailureDomainType string            `json:"failure_domain_type,omitempty"` // "" | "disk"
	DefaultDiskType   string            `json:"default_disk_type,omitempty"`   // YDB disk category, e.g. "SSD"
	StorageGroups     int               `json:"storage_groups,omitempty"`      // database create pool group count
	AutoSizePdisks    bool              `json:"auto_size_pdisks,omitempty"`    // dry-run may resize secondary disks from workload size
	DatabasePath      string            `json:"database_path"`                 // default "/Root/testdb"
	StorageOptions    map[string]string `json:"storage_options,omitempty"`
	DatabaseOptions   map[string]string `json:"database_options,omitempty"`
	HAProxyOptions    map[string]string `json:"haproxy_options,omitempty"`
}

// YDBManagedKind selects the Yandex Cloud Managed YDB flavor.
type YDBManagedKind string

const (
	YDBManagedKindServerless YDBManagedKind = "serverless"
	YDBManagedKindDedicated  YDBManagedKind = "dedicated"
)

// YDBManagedComputeType selects the workload class shown in the YC console.
// It is purely a UI / preset-filter affordance: the YDB terraform provider
// has no OLTP/OLAP cluster flag; what changes between modes is the set of
// `resource_preset_id` values you pick from. Some presets ("oltp-c16-m128")
// are explicitly tuned for the OLTP workload, others are general-purpose.
type YDBManagedComputeType string

const (
	YDBManagedComputeOLTP YDBManagedComputeType = "oltp"
	YDBManagedComputeOLAP YDBManagedComputeType = "olap"
)

// YDBManagedAutoScale enables and parameterises the dedicated cluster's
// auto-scaling. Mirrors `scale_policy.auto_scale` on the terraform side.
// The provider treats auto-scaling as a preview feature gated by the
// `enable_autoscaling=1` label (set automatically when this is non-nil).
type YDBManagedAutoScale struct {
	MinSize           int `json:"min_size"`
	MaxSize           int `json:"max_size"`
	CPUUtilizationPct int `json:"cpu_utilization_percent,omitempty"`
}

// YDBManagedTopology describes a Yandex Cloud Managed YDB deployment plus
// the client VM that runs stroppy. There are no storage / compute nodes —
// YC manages the database. The client VM is provisioned with an attached
// service account so the patched stroppy driver can pull SA token + CA from
// the YC metadata service (see pkg/driver/ydb/driver.go fallback path).
type YDBManagedTopology struct {
	// Type selects serverless (pay-per-request, grpc only) vs dedicated
	// (fixed cluster, grpc + pgwire). Required.
	Type YDBManagedKind `json:"type"`
	// ComputeType is the workload class shown in the YC console (oltp /
	// olap). Persisted so the UI can filter the resource_preset_id list
	// to the matching family on edit; not forwarded to terraform.
	ComputeType YDBManagedComputeType `json:"compute_type,omitempty"`
	// ResourcePresetID is the dedicated DB resource preset (e.g.
	// "medium"). Ignored for serverless. See types.YDBManagedResourcePresets.
	ResourcePresetID string `json:"resource_preset_id,omitempty"`
	// NodeCount is the dedicated DB fixed-scale node count. Ignored when
	// AutoScale is set. Defaults to 1.
	NodeCount int `json:"node_count,omitempty"`
	// AutoScale, if non-nil, switches the dedicated cluster to auto-scale
	// mode and replaces NodeCount. Provider gates this behind a label.
	AutoScale *YDBManagedAutoScale `json:"auto_scale,omitempty"`
	// StorageGroups is the dedicated DB number of storage groups. Ignored
	// for serverless.
	StorageGroups int `json:"storage_groups,omitempty"`
	// StorageType is the dedicated DB disk type ID (e.g. "ssd"). Ignored
	// for serverless.
	StorageType string `json:"storage_type,omitempty"`
	// Throttling for serverless. Optional; YC defaults applied when zero.
	ThrottlingRCUs int `json:"throttling_rcus,omitempty"`
	// Client is the stroppy runner VM spec. Provisioned with an attached
	// service account that has the ydb.editor role.
	Client MachineSpec `json:"client"`
	// DatabasePath is filled in by the machines task from terraform output
	// after apply (e.g. "/ru-central1/<cloud>/<db>"). Not set by users.
	DatabasePath string `json:"database_path,omitempty"`
	// Endpoint is filled in by the machines task from terraform output
	// (e.g. "ydb.serverless.yandexcloud.net:2135"). Not set by users.
	Endpoint string `json:"endpoint,omitempty"`
	// TerraformOutput is the full attribute snapshot of the managed YDB
	// resource as returned by terraform (id, status, full endpoints,
	// labels, etc.). Filled by the machines task after apply; surfaced in
	// the run overview so users see the deployment as it actually landed.
	TerraformOutput *YDBManagedTerraformOutput `json:"terraform_output,omitempty"`
}

// YDBManagedTerraformOutput mirrors the `ydb_managed` terraform output of
// deployments/terraform/yandex_managed_ydb. Pointer-typed fields apply only
// to one of the two flavours (serverless vs dedicated).
type YDBManagedTerraformOutput struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Type                string            `json:"type"` // "serverless" | "dedicated"
	FolderID            string            `json:"folder_id"`
	LocationID          string            `json:"location_id"`
	DatabasePath        string            `json:"database_path"`
	YDBAPIEndpoint      string            `json:"ydb_api_endpoint"`
	YDBFullEndpoint     string            `json:"ydb_full_endpoint"`
	DocumentAPIEndpoint string            `json:"document_api_endpoint"`
	TLSEnabled          bool              `json:"tls_enabled"`
	Status              string            `json:"status"`
	CreatedAt           string            `json:"created_at"`
	Labels              map[string]string `json:"labels,omitempty"`
	ResourcePresetID    string            `json:"resource_preset_id,omitempty"`   // dedicated
	NetworkID           string            `json:"network_id,omitempty"`           // dedicated
	SubnetIDs           []string          `json:"subnet_ids,omitempty"`           // dedicated
	StorageGroups       int               `json:"storage_groups,omitempty"`       // dedicated
	StorageTypeID       string            `json:"storage_type_id,omitempty"`      // dedicated
	ThrottlingRCULimit  int               `json:"throttling_rcu_limit,omitempty"` // serverless
}

// PicodataTier describes a tier in a multi-tier Picodata deployment.
type PicodataTier struct {
	Name        string `json:"name"`
	Replication int    `json:"replication_factor"`
	CanVote     bool   `json:"can_vote"`
	Count       int    `json:"count"`
}

// DatabaseConfig holds the database specification.
// Exactly one topology field must be set, matching Kind.
type DatabaseConfig struct {
	Kind       DatabaseKind        `json:"kind"`
	Version    string              `json:"version"`
	Postgres   *PostgresTopology   `json:"postgres,omitempty"`
	MySQL      *MySQLTopology      `json:"mysql,omitempty"`
	MariaDB    *MySQLTopology      `json:"mariadb,omitempty"` // MariaDB is wire- and config-compatible with MySQL; reuse the topology shape
	Picodata   *PicodataTopology   `json:"picodata,omitempty"`
	YDB        *YDBTopology        `json:"ydb,omitempty"`
	YDBManaged *YDBManagedTopology `json:"ydb_managed,omitempty"`
	Cockroach  *CockroachTopology  `json:"cockroach,omitempty"`
	Testing    *TestingTopology    `json:"testing,omitempty"`
	// RenderedConfigOverrides lets the SPA submit raw config-file contents
	// that replace the per-component generators on the agent. Keys identify
	// the file by its on-host purpose (e.g. "postgresql.conf:master",
	// "postgresql.conf:replica", "pg_hba.conf", "my.cnf:primary",
	// "ydb.yaml:storage"). When set for a key, the agent writes the value
	// verbatim instead of rendering from defaults+options. The dry-run
	// preview ships baseline contents under the same keys so the SPA can
	// show diffable starting points.
	RenderedConfigOverrides map[string]string `json:"rendered_config_overrides,omitempty"`
}

// --- Database topology presets ---

// CockroachPreset identifies a CockroachDB topology preset.
type CockroachPreset string

const (
	CockroachSingle   CockroachPreset = "single"
	CockroachCluster3 CockroachPreset = "cluster-3"
	CockroachCluster6 CockroachPreset = "cluster-6"
)

// PostgresPreset identifies a Postgres topology preset.
type PostgresPreset string

const (
	PostgresSingle PostgresPreset = "single"
	PostgresHA     PostgresPreset = "ha"
	PostgresScale  PostgresPreset = "scale"
)

// MySQLPreset identifies a MySQL topology preset.
type MySQLPreset string

const (
	MySQLSingle  MySQLPreset = "single"
	MySQLReplica MySQLPreset = "replica"
	MySQLGroup   MySQLPreset = "group"
)

// PicodataPreset identifies a Picodata topology preset.
type PicodataPreset string

const (
	PicodataSingle  PicodataPreset = "single"
	PicodataCluster PicodataPreset = "cluster"
	PicodataScale   PicodataPreset = "scale"
)

// TestingPreset identifies a synthetic testing topology preset.
type TestingPreset string

const (
	TestingPresetNoopDriver TestingPreset = "noop-driver"
	TestingPresetPgNoop     TestingPreset = "pg-noop"
)

// PostgresPresets contains all available Postgres topology presets.
var PostgresPresets = map[PostgresPreset]PostgresTopology{
	PostgresSingle: {
		Master: MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
	},
	PostgresHA: {
		Master:           MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 4, MemoryMB: 8192, DiskGB: 100},
		Replicas:         []MachineSpec{{Role: RoleDatabase, Count: 2, CPUs: 4, MemoryMB: 8192, DiskGB: 100}},
		HAProxy:          &MachineSpec{Role: RoleProxy, Count: 1, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
		PgBouncer:        true,
		Patroni:          true,
		Etcd:             true,
		SyncReplicas:     1,
		MasterOptions:    map[string]string{"shared_buffers": "25%", "max_connections": "200", "work_mem": "64MB"},
		ReplicaOptions:   map[string]string{"shared_buffers": "25%", "max_connections": "200", "work_mem": "64MB"},
		PgBouncerOptions: map[string]string{"pool_mode": "transaction", "max_client_conn": "1000", "default_pool_size": "25"},
		PatroniOptions:   map[string]string{"ttl": "30", "loop_wait": "10", "retry_timeout": "10"},
	},
	PostgresScale: {
		Master:           MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 8, MemoryMB: 16384, DiskGB: 200},
		Replicas:         []MachineSpec{{Role: RoleDatabase, Count: 4, CPUs: 8, MemoryMB: 16384, DiskGB: 200}},
		HAProxy:          &MachineSpec{Role: RoleProxy, Count: 2, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
		PgBouncer:        true,
		Patroni:          true,
		Etcd:             true,
		SyncReplicas:     2,
		MasterOptions:    map[string]string{"shared_buffers": "25%", "max_connections": "500", "work_mem": "128MB", "effective_cache_size": "75%"},
		ReplicaOptions:   map[string]string{"shared_buffers": "25%", "max_connections": "500", "work_mem": "128MB", "effective_cache_size": "75%"},
		PgBouncerOptions: map[string]string{"pool_mode": "transaction", "max_client_conn": "2000", "default_pool_size": "50"},
		PatroniOptions:   map[string]string{"ttl": "30", "loop_wait": "10", "retry_timeout": "10"},
	},
}

// MySQLPresets contains all available MySQL topology presets.
var MySQLPresets = map[MySQLPreset]MySQLTopology{
	MySQLSingle: {
		Primary: MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
	},
	MySQLReplica: {
		Primary:         MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 4, MemoryMB: 8192, DiskGB: 100},
		Replicas:        []MachineSpec{{Role: RoleDatabase, Count: 2, CPUs: 4, MemoryMB: 8192, DiskGB: 100}},
		ProxySQL:        &MachineSpec{Role: RoleProxy, Count: 1, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
		SemiSync:        true,
		PrimaryOptions:  map[string]string{"innodb_buffer_pool_size": "25%", "max_connections": "200"},
		ReplicaOptions:  map[string]string{"innodb_buffer_pool_size": "25%", "max_connections": "200"},
		ProxySQLOptions: map[string]string{"threads": "4", "max_connections": "2048"},
	},
	MySQLGroup: {
		Primary:         MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 8, MemoryMB: 16384, DiskGB: 200},
		Replicas:        []MachineSpec{{Role: RoleDatabase, Count: 2, CPUs: 8, MemoryMB: 16384, DiskGB: 200}},
		ProxySQL:        &MachineSpec{Role: RoleProxy, Count: 2, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
		GroupRepl:       true,
		PrimaryOptions:  map[string]string{"innodb_buffer_pool_size": "25%", "max_connections": "500"},
		ReplicaOptions:  map[string]string{"innodb_buffer_pool_size": "25%", "max_connections": "500"},
		ProxySQLOptions: map[string]string{"threads": "4", "max_connections": "2048"},
	},
}

// PicodataPresets contains all available Picodata topology presets.
var PicodataPresets = map[PicodataPreset]PicodataTopology{
	PicodataSingle: {
		Instances:   []MachineSpec{{Role: RoleDatabase, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 50}},
		Replication: 1,
		Shards:      1,
	},
	PicodataCluster: {
		Instances:   []MachineSpec{{Role: RoleDatabase, Count: 3, CPUs: 4, MemoryMB: 8192, DiskGB: 100}},
		HAProxy:     &MachineSpec{Role: RoleProxy, Count: 1, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
		Replication: 2,
		Shards:      3,
	},
	PicodataScale: {
		Instances:   []MachineSpec{{Role: RoleDatabase, Count: 6, CPUs: 8, MemoryMB: 16384, DiskGB: 200}},
		HAProxy:     &MachineSpec{Role: RoleProxy, Count: 2, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
		Replication: 3,
		Shards:      6,
		Tiers: []PicodataTier{
			{Name: "compute", Replication: 1, CanVote: true, Count: 3},
			{Name: "storage", Replication: 2, CanVote: false, Count: 3},
		},
	},
}

// CockroachPresets contains all built-in CockroachDB topology presets. Every
// node is the same flavor — CockroachDB doesn't have a master/replica split,
// so the only knob is node count. 4 vCPU / 8 GB / 100 GB matches the official
// "starter cluster" sizing in CockroachLabs' docs.
var CockroachPresets = map[CockroachPreset]CockroachTopology{
	CockroachSingle: {
		Nodes: MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
	},
	CockroachCluster3: {
		Nodes: MachineSpec{Role: RoleDatabase, Count: 3, CPUs: 4, MemoryMB: 8192, DiskGB: 100},
	},
	CockroachCluster6: {
		Nodes: MachineSpec{Role: RoleDatabase, Count: 6, CPUs: 8, MemoryMB: 16384, DiskGB: 200},
	},
}

// TestingPresets contains built-in synthetic targets for smoke, runner, and
// network overhead testing.
var TestingPresets = map[TestingPreset]TestingTopology{
	TestingPresetNoopDriver: {
		Mode: TestingNoopDriver,
	},
	TestingPresetPgNoop: {
		Mode:     TestingPgNoop,
		Database: &MachineSpec{Role: RoleDatabase, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
		PgNoop:   &PgNoopConfig{Version: "0.1.1", Port: 5432},
	},
}

// --- Monitoring ---

// MonitorConfig holds monitoring export targets.
// Monitoring is always deployed; this configures where to send data.
type MonitorConfig struct {
	MetricsEndpoint string `json:"metrics_endpoint,omitempty"` // Prometheus remote_write URL
	LogsEndpoint    string `json:"logs_endpoint,omitempty"`    // Loki push URL
}

// WorkloadFile is a run-scoped file that must be present in stroppy's working
// directory when probing or running a workload. It is intentionally embedded in
// the run config instead of persisted as a reusable catalog entry.
type WorkloadFile struct {
	Name    string `json:"name"`
	Kind    string `json:"kind,omitempty"` // currently "sql"; left open for future script/support files
	Content string `json:"content"`
}

// StroppyConfig holds stroppy test runner settings.
type StroppyConfig struct {
	Version string `json:"version"` // stroppy binary version (e.g. "4.1.0")
	// Protocol selects the wire format stroppy uses to talk to the database.
	// When unset, defaults to types.DefaultProtocol(database.kind) — preserves
	// behaviour of pre-protocol-aware run configs. For engines that speak
	// only one protocol (postgres / mysql / mariadb / picodata) leaving this
	// blank is fine; for YDB the choice matters (ydb-grpc vs ydb-pgwire).
	Protocol            Protocol          `json:"protocol,omitempty"`
	Script              string            `json:"script"`                          // e.g. "tpcc/procs", "tpcc/tx", "tpcb/procs", "tpcb/tx"
	SQL                 string            `json:"sql,omitempty"`                   // optional second stroppy positional arg / RunConfig.sql
	Duration            string            `json:"duration"`                        // k6 --duration flag
	K6Mode              string            `json:"k6_mode,omitempty"`               // "duration" or "iterations"; defaults to duration
	Iterations          int               `json:"iterations,omitempty"`            // k6 --iterations flag when k6_mode=iterations
	Quiet               *bool             `json:"quiet,omitempty"`                 // k6 -q flag; nil preserves the UI default (enabled)
	NoThresholds        bool              `json:"no_thresholds,omitempty"`         // k6 --no-thresholds flag
	VUs                 int               `json:"vus,omitempty"`                   // k6 --vus flag
	PoolSize            int               `json:"pool_size,omitempty"`             // DB connection pool size → env POOL_SIZE + driver pool
	ScaleFactor         int               `json:"scale_factor,omitempty"`          // Warehouses → env SCALE_FACTOR
	DefaultInsertMethod string            `json:"default_insert_method,omitempty"` // driver defaultInsertMethod; "native" unless overridden
	Env                 map[string]string `json:"env,omitempty"`                   // script-specific env overrides from probe metadata
	Files               []WorkloadFile    `json:"files,omitempty"`
	Steps               []string          `json:"steps,omitempty"`    // step allowlist (e.g. ["create_schema","load_data","workload"])
	NoSteps             []string          `json:"no_steps,omitempty"` // step blocklist (e.g. ["drop_schema"])
	// ConfigOverrideJSON, if set, is sent verbatim to the stroppy binary instead of the
	// config built from the other fields. Allows advanced users to edit the full stroppy
	// RunConfig protojson (drivers, k6_args, env, exporter, etc.) before launching a run.
	ConfigOverrideJSON string `json:"config_override_json,omitempty"`
	// Machine spec for the stroppy runner node. If nil, defaults to 2 vCPU / 4 GB / 20 GB.
	Machine *MachineSpec `json:"machine,omitempty"`
	// Deprecated fields kept for backward compatibility with existing runs.
	Workload string  `json:"workload,omitempty"`  // Deprecated: use Script
	VUSScale float64 `json:"vus_scale,omitempty"` // Deprecated: use VUs
	Workers  int     `json:"workers,omitempty"`   // Deprecated: use VUs
}

// NetworkConfig holds network/subnet allocation settings.
type NetworkConfig struct {
	CIDR string `json:"cidr"`
	Zone string `json:"zone,omitempty"`
}

// RunConfig is the full specification of a test run.
// It is used to build the execution DAG.
type RunConfig struct {
	ID string `json:"id"`
	// Name is an optional human-friendly label for this run, set on creation
	// only. Surfaces in run lists / details. Empty string means unnamed.
	Name string `json:"name,omitempty"`
	// Description is an optional free-form note about this run. Set on
	// creation only.
	Description string         `json:"description,omitempty"`
	Provider    Provider       `json:"provider"`
	Network     NetworkConfig  `json:"network"`
	Machines    []MachineSpec  `json:"machines"`
	Database    DatabaseConfig `json:"database"`
	Monitor     MonitorConfig  `json:"monitor"`
	Stroppy     StroppyConfig  `json:"stroppy"`
	// PresetID references a presets row. If set and no topology is provided in Database,
	// the preset's topology is applied. Topology in the request takes priority.
	PresetID string `json:"preset_id,omitempty"`
	// PackageID references a packages row. Resolved to ResolvedPackage at run start.
	// If empty, the default built-in package for db_kind+version is used.
	PackageID string `json:"package_id,omitempty"`
	// PlatformID overrides the Yandex Cloud platform from server settings (e.g. "standard-v3").
	PlatformID string `json:"platform_id,omitempty"`
	// MachineOverride, when set, overrides the CPU/memory/disk of all database-role
	// machines from the preset topology. Allows per-run sizing without editing the preset.
	MachineOverride *MachineSpec `json:"machine_override,omitempty"`
	// RunPresetID references a run_presets row when this run was started from
	// a saved run-preset (workload+infra parameter template). Surfaced in the
	// run summary; resolution happens client-side.
	RunPresetID string `json:"run_preset_id,omitempty"`
	// SuiteID references a suite_runs row when this run was started as part
	// of a suite (a sequence of run-presets). Surfaced in the run summary so
	// the UI can group runs by suite.
	SuiteID string `json:"suite_id,omitempty"`
	// ExternalDB, when set, treats the run as bring-your-own-database: the
	// agent skips infra/install/configure phases and points stroppy at the
	// supplied endpoint. Database.Kind / Database.Version still describe the
	// target so script compatibility checks work.
	ExternalDB *ExternalDBConfig `json:"external_db,omitempty"`
	// ResolvedPackage is populated by the server before building the DAG.
	// Serialised so the resolution survives the durable job_runs queue —
	// scheduler claims a row, unmarshals the cfg, and the install task
	// downstream needs Package non-nil. Clients sending this field have it
	// overwritten by resolveRunPackage on the server anyway.
	ResolvedPackage *Package `json:"resolved_package,omitempty"`
}

// ExternalDBConfig describes a user-supplied database endpoint. When present
// in RunConfig, the executor skips machines/install/configure phases for the
// database and routes stroppy directly at this endpoint. Monitoring is also
// skipped (no agents to scrape) — metrics come only from stroppy itself.
type ExternalDBConfig struct {
	// Endpoint is the connection target as a host:port pair (e.g.
	// "10.1.0.5:5432"). For YDB this is the grpc endpoint.
	Endpoint string `json:"endpoint"`
	// Database is the database name / path (e.g. "stroppy", or for YDB
	// "/Root/testdb").
	Database string `json:"database,omitempty"`
	// Username and password for authentication. Optional for engines that
	// support trust auth (rare in production).
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// SSLMode is engine-specific (e.g. "disable", "require"). Optional.
	SSLMode string `json:"ssl_mode,omitempty"`
}
