package run

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbconfig"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

type proxyInstallTask struct {
	client agent.Client
	state  *State
	dbKind types.DatabaseKind
}

func (t *proxyInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.ProxyTargets()
	if len(targets) == 0 {
		nc.Log().Info("no proxy targets, skipping install")
		return nil
	}

	switch t.dbKind {
	case types.DatabasePostgres, types.DatabasePicodata, types.DatabaseYDB:
		nc.Log().Info("installing haproxy")
		// Old installHAProxy: aptInstall("haproxy").
		return sendSeqAll(nc, t.client, targets,
			aptCmd("install_haproxy", `DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends haproxy`))
	case types.DatabaseMySQL, types.DatabaseMariaDB:
		nc.Log().Info("installing proxysql")
		// Old installProxySQL: download the .deb from GitHub releases first
		// (more reliable than repo.proxysql.com); fall back to the apt repo if
		// the GitHub download fails. Keep both paths.
		version := "2.7.3"
		url := fmt.Sprintf("https://github.com/sysown/proxysql/releases/download/v%s/proxysql_%s-ubuntu22_amd64.deb", version, version)
		githubScript := fmt.Sprintf(`curl -fsSL --connect-timeout 20 --max-time 120 --retry 3 --retry-delay 5 --retry-connrefused --retry-max-time 300 "%s" -o /tmp/proxysql.deb && dpkg -i /tmp/proxysql.deb && rm -f /tmp/proxysql.deb`, url)

		// Fallback: use apt repo if GitHub download fails.
		fallbackScript := `wget -qO - https://repo.proxysql.com/ProxySQL/repo_pub_key | apt-key add -
echo "deb https://repo.proxysql.com/ProxySQL/proxysql-2.7.x/$(lsb_release -sc)/ ./" > /etc/apt/sources.list.d/proxysql.list
apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends proxysql`

		return sendSeqAll(nc, t.client, targets,
			runCmd("install_proxysql", githubScript),
			aptCmd("install_proxysql", fallbackScript))
	default:
		return nil
	}
}

type proxyConfigTask struct {
	client        agent.Client
	state         *State
	dbKind        types.DatabaseKind
	pgTopology    *types.PostgresTopology
	mysqlTopology *types.MySQLTopology
	picoTopology  *types.PicodataTopology
	ydbTopology   *types.YDBTopology
	overrides     map[string]string // DatabaseConfig.RenderedConfigOverrides — keys: "haproxy.cfg", "proxysql.cnf"
}

func (t *proxyConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.ProxyTargets()
	if len(targets) == 0 {
		nc.Log().Info("no proxy targets, skipping config")
		return nil
	}

	dbTargets := t.state.DBTargets()

	switch t.dbKind {
	case types.DatabasePostgres:
		return t.configHAProxyPostgres(nc, targets, dbTargets)
	case types.DatabaseMySQL, types.DatabaseMariaDB:
		return t.configProxySQLMySQL(nc, targets, dbTargets)
	case types.DatabasePicodata:
		return t.configHAProxyPicodata(nc, targets, dbTargets)
	case types.DatabaseYDB:
		return t.configHAProxyYDB(nc, targets, dbTargets)
	default:
		return nil
	}
}

// haproxyCmds renders the haproxy.cfg body (user override wins, else render
// server-side) and returns the write + start commands. Mirrors the old
// configHAProxy agent method.
func (t *proxyConfigTask) haproxyCmds(opts dbconfig.RenderHAProxyConfOpts) []agent.Command {
	body := t.overrides["haproxy.cfg"]
	if body == "" {
		body = dbconfig.RenderHAProxyConf(opts)
	}
	return []agent.Command{
		writeFile("config_haproxy", "/etc/haproxy/haproxy.cfg", body),
		runCmd("config_haproxy", "haproxy -f /etc/haproxy/haproxy.cfg -D"),
	}
}

func (t *proxyConfigTask) configHAProxyPostgres(nc *dag.NodeContext, proxyTargets, dbTargets []agent.Target) error {
	nc.Log().Info("configuring haproxy for postgres (patroni health checks)")

	var backends []string
	for _, tgt := range dbTargets {
		backends = append(backends, fmt.Sprintf("%s:5432", advertiseHost(tgt)))
	}

	healthCheck := "tcp"
	patroniPort := 0
	if t.pgTopology != nil && t.pgTopology.Patroni {
		healthCheck = "patroni"
		patroniPort = 8008
	}

	cmds := t.haproxyCmds(dbconfig.RenderHAProxyConfOpts{
		WritePort:   5000,
		ReadPort:    5001,
		Backends:    backends,
		HealthCheck: healthCheck,
		PatroniPort: patroniPort,
	})
	return sendSeqAll(nc, t.client, proxyTargets, cmds...)
}

func (t *proxyConfigTask) configProxySQLMySQL(nc *dag.NodeContext, proxyTargets, dbTargets []agent.Target) error {
	nc.Log().Info("configuring proxysql for mysql")

	var backends []string
	for _, tgt := range dbTargets {
		backends = append(backends, fmt.Sprintf("%s:3306", advertiseHost(tgt)))
	}

	// Config body: user override wins, else render server-side. The agent used
	// to substitute __PROXYSQL_BACKEND_HOST_<i>__ placeholders — do that here.
	body := t.overrides["proxysql.cnf"]
	if body == "" {
		body = dbconfig.RenderProxySQLConf(dbconfig.RenderProxySQLConfOpts{
			BackendCount:    len(backends),
			ListenPort:      6033,
			AdminPort:       6032,
			WriterHostgroup: 10,
			ReaderHostgroup: 20,
		})
	}
	body = dbconfig.SubstituteProxySQLBackends(body, backends)

	cmds := []agent.Command{
		writeFile("config_proxysql", "/etc/proxysql.cnf", body),
		runCmd("config_proxysql", "mkdir -p /var/lib/proxysql"),
		startDaemonCmd("config_proxysql", "proxysql", "proxysql",
			[]string{"--initial", "-f", "-D", "/var/lib/proxysql", "-c", "/etc/proxysql.cnf"}, nil),
	}
	return sendSeqAll(nc, t.client, proxyTargets, cmds...)
}

func (t *proxyConfigTask) configHAProxyPicodata(nc *dag.NodeContext, proxyTargets, dbTargets []agent.Target) error {
	nc.Log().Info("configuring haproxy for picodata (pgproto)")

	var backends []string
	for _, tgt := range dbTargets {
		backends = append(backends, fmt.Sprintf("%s:4327", advertiseHost(tgt)))
	}

	cmds := t.haproxyCmds(dbconfig.RenderHAProxyConfOpts{
		WritePort:   4327,
		ReadPort:    4328,
		Backends:    backends,
		HealthCheck: "tcp",
	})
	return sendSeqAll(nc, t.client, proxyTargets, cmds...)
}

func (t *proxyConfigTask) configHAProxyYDB(nc *dag.NodeContext, proxyTargets, dbTargets []agent.Target) error {
	nc.Log().Info("configuring haproxy for YDB (gRPC)")

	var backends []string
	for _, tgt := range dbTargets {
		backends = append(backends, fmt.Sprintf("%s:2136", advertiseHost(tgt)))
	}

	cmds := t.haproxyCmds(dbconfig.RenderHAProxyConfOpts{
		WritePort:   2136,
		ReadPort:    2137,
		Backends:    backends,
		HealthCheck: "tcp",
	})
	return sendSeqAll(nc, t.client, proxyTargets, cmds...)
}
