package agent

// YDBInstallConfig is the agent payload for YDB binary installation.
type YDBInstallConfig struct {
	Version string `json:"version"`
}

// YDBStaticConfig is the agent payload for starting a YDB static (storage) node.
type YDBStaticConfig struct {
	Hosts          []string          `json:"hosts"` // all static node addresses
	InstanceID     int               `json:"instance_id"`
	AdvertiseHost  string            `json:"advertise_host"`
	DiskPath       string            `json:"disk_path"`                  // "/ydb_data" in Docker / file-backed pdisk
	BlockDevicePath string           `json:"block_device_path,omitempty"` // when set, pdisk is the raw device — agent skips truncate / mkdir and obliterates the device directly
	DiskGB         int               `json:"disk_gb"`   // allocated disk size; pdisk is sized to this when file-backed
	MemoryMB       int               `json:"memory_mb"` // total machine RAM for memory limits
	CPUs           int               `json:"cpus"`      // vCPUs for actor system tuning
	FaultTolerance string            `json:"fault_tolerance"`
	Options        map[string]string `json:"options,omitempty"`
	// ConfOverride, when non-empty, replaces the rendered YDB config.yaml
	// body. The agent still substitutes __YDB_HOST_<i>__ placeholders with
	// the entries of Hosts before writing the file.
	ConfOverride string `json:"conf_override,omitempty"`
}

// YDBInitConfig is the agent payload for cluster initialization (runs on one static node).
type YDBInitConfig struct {
	StaticEndpoint string `json:"static_endpoint"` // grpc://<host>:2136
	DatabasePath   string `json:"database_path"`   // /Root/testdb
	ConfigPath     string `json:"config_path"`     // /opt/ydb/cfg/config.yaml
}

// YDBDatabaseConfig is the agent payload for starting a YDB dynamic (database) node.
type YDBDatabaseConfig struct {
	StaticEndpoints []string          `json:"static_endpoints"` // node-broker addresses
	AdvertiseHost   string            `json:"advertise_host"`
	DatabasePath    string            `json:"database_path"` // /Root/testdb
	MemoryMB        int               `json:"memory_mb"`     // total machine RAM for memory limits
	CPUs            int               `json:"cpus"`          // vCPUs for actor system tuning
	FaultTolerance  string            `json:"fault_tolerance,omitempty"`
	StorageHosts    []string          `json:"storage_hosts,omitempty"`     // mirrors YDBStaticConfig.Hosts; needed to substitute placeholders in the database yaml
	BlockDevicePath string            `json:"block_device_path,omitempty"` // mirrors the storage value so the database yaml's path: lines stay consistent with the cluster's
	Options         map[string]string `json:"options,omitempty"`
	// ConfOverride, when non-empty, replaces the rendered ydb-database.yaml
	// body for this node. The agent still substitutes __YDB_HOST_<i>__
	// placeholders from StorageHosts.
	ConfOverride string `json:"conf_override,omitempty"`
}
