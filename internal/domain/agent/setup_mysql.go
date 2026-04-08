package agent

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// MySQLInstallConfig is the agent payload for MySQL installation.
type MySQLInstallConfig struct {
	Version string         `json:"version"`
	DataDir string         `json:"data_dir"`
	Package *types.Package `json:"package,omitempty"`
}

// MySQLClusterConfig is the agent payload for MySQL cluster setup.
type MySQLClusterConfig struct {
	Role        string            `json:"role"` // "primary" or "replica"
	PrimaryHost string            `json:"primary_host,omitempty"`
	LocalHost   string            `json:"local_host,omitempty"` // this node's address (for GR local_address)
	NodeIndex   int               `json:"node_index"`           // 0-based index for server-id generation
	SemiSync    bool              `json:"semi_sync"`            // enable semi-synchronous replication
	GroupRepl   bool              `json:"group_replication"`
	GroupSeeds  []string          `json:"group_seeds,omitempty"` // all host:33061 addresses for GR
	GroupName   string            `json:"group_name,omitempty"`  // UUID for group_replication_group_name
	Options     map[string]string `json:"options,omitempty"`
}
