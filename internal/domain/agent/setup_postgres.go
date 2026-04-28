package agent

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// PostgresInstallConfig is the agent payload for postgres installation.
type PostgresInstallConfig struct {
	Version string         `json:"version"`
	DataDir string         `json:"data_dir"`
	Package *types.Package `json:"package,omitempty"`
}

// PostgresClusterConfig is the agent payload for postgres cluster setup.
type PostgresClusterConfig struct {
	Version      string            `json:"version"`
	Role         string            `json:"role"` // "master" or "replica"
	MasterHost   string            `json:"master_host,omitempty"`
	Patroni      bool              `json:"patroni"`
	SyncReplicas int               `json:"sync_replicas"`
	MemoryMB     int               `json:"memory_mb,omitempty"` // budget for "25%"-style defaults; falls back to /proc/meminfo when 0
	Options      map[string]string `json:"options,omitempty"`
	// ConfOverride, when non-empty, replaces the rendered postgresql.conf body.
	// Set on the run side from DatabaseConfig.RenderedConfigOverrides for the
	// "postgresql.conf:<role>" key when the user edited it on the review step.
	ConfOverride string `json:"conf_override,omitempty"`
	// HBAOverride does the same for pg_hba.conf.
	HBAOverride string `json:"hba_override,omitempty"`
}
