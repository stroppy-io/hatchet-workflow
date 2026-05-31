package run

import (
	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbconfig"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// pgBouncerInstallTask installs the pgbouncer package on every DB node.
// PgBouncer is colocated on all DB nodes; the agent only runs the opaque apt
// script the server composes here.
type pgBouncerInstallTask struct {
	client agent.Client
	state  *State
}

func (t *pgBouncerInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("installing pgbouncer on DB nodes")
	// Ported verbatim from the old agent installPgBouncer / aptInstall("pgbouncer").
	return sendSeqAll(nc, t.client, targets,
		aptCmd("install_pgbouncer",
			"DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends pgbouncer"))
}

// pgBouncerConfigTask renders pgbouncer.ini, writes userlist.txt and starts the
// pooler. All rendering happens here; the agent just writes files and runs the
// start script.
type pgBouncerConfigTask struct {
	client    agent.Client
	state     *State
	topology  *types.PostgresTopology
	overrides map[string]string // DatabaseConfig.RenderedConfigOverrides — keys: "pgbouncer.ini"
}

func (t *pgBouncerConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring pgbouncer")

	// Default auth_type is "trust" for testing; PgBouncerOptions can override it.
	authType := "trust"
	if t.topology != nil {
		if v, ok := t.topology.PgBouncerOptions["auth_type"]; ok && v != "" {
			authType = v
		}
	}

	const (
		listenPort      = 6432
		poolMode        = "transaction"
		maxClientConn   = 1000
		defaultPoolSize = 25
		pgPort          = 5432
	)
	// PgBouncer is colocated with postgres, so it always connects to the local
	// instance — the same default the old task shipped via PGHost.
	const pgHost = "127.0.0.1"

	// Config body: user override wins, else render server-side. The placeholder
	// substitution the old agent did also moves here.
	body := t.overrides["pgbouncer.ini"]
	if body == "" {
		body = dbconfig.RenderPgBouncerConf(dbconfig.RenderPgBouncerConfOpts{
			ListenPort:      listenPort,
			PoolMode:        poolMode,
			MaxClientConn:   maxClientConn,
			DefaultPoolSize: defaultPoolSize,
			PGPort:          pgPort,
			AuthType:        authType,
		})
	}
	body = dbconfig.SubstitutePgBouncerPlaceholders(body, pgHost)

	// userlist.txt (trust mode, empty passwords).
	const userlist = "\"postgres\" \"\"\n"

	// Create the pgbouncer user/dirs and start the daemon. Ported verbatim from
	// the old agent configPgBouncer start script.
	startScript := `id -u pgbouncer >/dev/null 2>&1 || useradd -r -m -s /bin/false pgbouncer && ` +
		`mkdir -p /var/run/pgbouncer /var/log/pgbouncer && ` +
		`chown -R pgbouncer:pgbouncer /etc/pgbouncer /var/run/pgbouncer /var/log/pgbouncer 2>/dev/null; ` +
		`su -s /bin/bash pgbouncer -c "pgbouncer -d /etc/pgbouncer/pgbouncer.ini"`

	return sendSeqAll(nc, t.client, targets,
		writeFile("config_pgbouncer", "/etc/pgbouncer/pgbouncer.ini", body),
		writeFile("config_pgbouncer", "/etc/pgbouncer/userlist.txt", userlist),
		runCmd("config_pgbouncer", startScript),
	)
}
