package agent

// ProxySQLInstallConfig is the agent payload for ProxySQL installation.
type ProxySQLInstallConfig struct{}

// ProxySQLConfig is the agent payload for ProxySQL configuration.
type ProxySQLConfig struct {
	ListenPort       int      `json:"listen_port"`       // default 6033
	AdminPort        int      `json:"admin_port"`        // default 6032
	Backends         []string `json:"backends"`          // host:port list
	GroupReplication bool     `json:"group_replication"` // use GR hostgroup auto-management
	WriterHostgroup  int      `json:"writer_hostgroup"`  // default 10
	ReaderHostgroup  int      `json:"reader_hostgroup"`  // default 20
	// ConfOverride, when non-empty, replaces the rendered proxysql.cnf body.
	// Set on the run side from DatabaseConfig.RenderedConfigOverrides for
	// the "proxysql.cnf" key when the user edited it on the review step.
	// The agent still substitutes __PROXYSQL_BACKEND_HOST_<i>__ placeholders
	// from Backends before writing.
	ConfOverride string `json:"conf_override,omitempty"`
}
