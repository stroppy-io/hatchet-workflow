package agent

// PgNoopInstallConfig is the agent payload for pg-noop binary installation.
type PgNoopInstallConfig struct {
	Version string `json:"version"`
}

// PgNoopConfig is the agent payload for pg-noop runtime configuration.
type PgNoopConfig struct {
	Host    string `json:"host,omitempty"`
	Port    int    `json:"port,omitempty"`
	Workers int    `json:"workers,omitempty"`
}
