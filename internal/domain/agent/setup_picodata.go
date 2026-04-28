package agent

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// PicodataInstallConfig is the agent payload for Picodata installation.
type PicodataInstallConfig struct {
	Version string         `json:"version"`
	DataDir string         `json:"data_dir"`
	Package *types.Package `json:"package,omitempty"`
}

// PicodataClusterConfig is the agent payload for Picodata cluster setup.
type PicodataClusterConfig struct {
	InstanceID    int               `json:"instance_id"`
	Peers         []string          `json:"peers"`                    // addresses of all instances
	AdvertiseHost string            `json:"advertise_host,omitempty"` // IP/hostname for advertise; falls back to os.Hostname()
	Replication   int               `json:"replication_factor"`
	Shards        int               `json:"shards"`
	MemoryMB      int               `json:"memory_mb,omitempty"` // budget for "25%"-style defaults; falls back to /proc/meminfo when 0
	Options       map[string]string `json:"options,omitempty"`
	// ConfOverride, when non-empty, replaces the rendered picodata.yaml body.
	// Set on the run side from DatabaseConfig.RenderedConfigOverrides for
	// "picodata.yaml". The agent still substitutes the AdvertiseHost
	// placeholder per-instance.
	ConfOverride string `json:"conf_override,omitempty"`
}
