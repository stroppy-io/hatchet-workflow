package agent

// CockroachInstallConfig is the agent payload for installing the cockroach
// binary (tarball pull from binaries.cockroachdb.com).
type CockroachInstallConfig struct {
	Version string `json:"version"` // e.g. "24.2.4" — full patch version, falls back to a default when empty
}

// CockroachClusterConfig is the agent payload for starting a single
// CockroachDB node. Each node in the cluster gets one of these; init runs
// separately on the first node.
type CockroachClusterConfig struct {
	NodeIndex     int      `json:"node_index"`
	AdvertiseHost string   `json:"advertise_host"`          // this node's address as the cluster sees it
	Peers         []string `json:"peers,omitempty"`         // all node hostnames in the cluster, used for --join
	CacheMB       int      `json:"cache_mb,omitempty"`      // --cache flag (default 25% of MemoryMB)
	SQLMemoryMB   int      `json:"sql_memory_mb,omitempty"` // --max-sql-memory flag (default 25% of MemoryMB)
	MemoryMB      int      `json:"memory_mb,omitempty"`     // total machine memory; cache and sql defaults derive from this
}

// CockroachInitConfig is the agent payload for the one-shot cluster bootstrap
// (`cockroach init`). Only sent to the first node after all nodes are up.
type CockroachInitConfig struct {
	Host string `json:"host"`           // first node's address
	Port int    `json:"port,omitempty"` // defaults to 26257
	// ClusterSettings is applied as `SET CLUSTER SETTING <k> = <v>` after
	// init. Comes from CockroachTopology.Options.
	ClusterSettings map[string]string `json:"cluster_settings,omitempty"`
}
