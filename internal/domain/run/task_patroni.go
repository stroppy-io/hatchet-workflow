package run

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbconfig"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// noopTask is a DAG task that does nothing (used as placeholder when a phase
// is logically replaced by another phase but must still exist in the graph).
type noopTask struct{}

func (t *noopTask) Execute(_ *dag.NodeContext) error { return nil }

// patroniInstallTask installs Patroni on all DB nodes. The agent only runs the
// opaque apt/pip scripts the server composes here.
type patroniInstallTask struct {
	client agent.Client
	state  *State
}

func (t *patroniInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("installing patroni on DB nodes")
	// Both ops are package installs → exclusive (serialize on the apt/dpkg lock).
	// The pip install is also wrapped in aptCmd to serialize against apt.
	return sendSeqAll(nc, t.client, targets,
		aptCmd("install_patroni", "DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends python3-pip python3-dev libpq-dev"),
		aptCmd("install_patroni", "pip3 install patroni[etcd3] psycopg2-binary"),
	)
}

// patroniConfigTask configures Patroni on all DB nodes. All rendering and
// placeholder substitution happens here; the agent just writes the config file
// and runs the start/readiness scripts.
type patroniConfigTask struct {
	client    agent.Client
	state     *State
	version   string
	topology  *types.PostgresTopology
	overrides map[string]string // DatabaseConfig.RenderedConfigOverrides — keys: "patroni.yml"
}

func (t *patroniConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring patroni cluster")

	// Build etcd hosts string from first 3 DB targets (etcd is colocated on DB nodes).
	etcdTargets := targets
	if len(etcdTargets) > 3 {
		etcdTargets = etcdTargets[:3]
	}
	var etcdParts []string
	for _, tgt := range etcdTargets {
		etcdParts = append(etcdParts, fmt.Sprintf("%s:2379", advertiseHost(tgt)))
	}
	etcdHosts := strings.Join(etcdParts, ",")

	pgVersion := t.version
	if pgVersion == "" {
		pgVersion = "16"
	}

	syncMode := t.topology.SyncReplicas > 0

	dataDir := fmt.Sprintf("/var/lib/postgresql/%s/main", pgVersion)
	confPath := "/etc/patroni/patroni.yml"

	for i, target := range targets {
		name := "stroppy-pg"
		nodeName := fmt.Sprintf("pg%d", i)
		connectAddr := advertiseHost(target)

		// Config body: user override wins, else render server-side.
		body := t.overrides["patroni.yml"]
		if body == "" {
			body = dbconfig.RenderPatroniConf(dbconfig.RenderPatroniConfOpts{
				PGVersion: pgVersion,
				SyncMode:  syncMode,
				SyncCount: t.topology.SyncReplicas,
				PGOptions: t.topology.MasterOptions,
			})
		}
		body = dbconfig.SubstitutePatroniPlaceholders(body, name, nodeName, connectAddr, etcdHosts)

		cmds := []agent.Command{
			runCmd("config_patroni", "mkdir -p /etc/patroni"),
			writeFile("config_patroni", confPath, body),
			// Stop PG if apt started it — Patroni manages PG lifecycle.
			runCmd("config_patroni", fmt.Sprintf("systemctl stop postgresql 2>/dev/null; pg_ctlcluster %s main stop 2>/dev/null || true", pgVersion)),
			// Clear data dir so Patroni can bootstrap fresh.
			runCmd("config_patroni", fmt.Sprintf("rm -rf %s/*", dataDir)),
			runCmd("config_patroni", fmt.Sprintf("mkdir -p %s && chown postgres:postgres %s", dataDir, dataDir)),
			// Start Patroni via systemd-run as postgres user (initdb cannot run as root).
			runCmd("config_patroni", "systemctl stop patroni 2>/dev/null; systemctl reset-failed patroni 2>/dev/null"),
			runCmd("config_patroni", fmt.Sprintf(
				`systemd-run --unit=patroni --uid=postgres --gid=postgres -- patroni %s`, confPath)),
			// Wait for Patroni REST API to be ready (indicates PG is up and cluster is formed).
			runCmd("config_patroni", `for i in $(seq 1 30); do curl -sf http://localhost:8008/health 2>/dev/null && exit 0; sleep 2; done; echo "patroni not ready after 60s" >&2; journalctl -u patroni --no-pager -n 50 >&2; exit 1`),
		}

		if err := sendSeq(nc, t.client, target, cmds...); err != nil {
			return err
		}
	}
	return nil
}
