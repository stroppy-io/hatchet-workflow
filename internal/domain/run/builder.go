package run

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// Deps holds the dependencies the recipe + infra phases need. Client is the
// command SINK (the BuildRecipe plan collector); Deployer/Settings drive the
// real deploy/teardown side effects.
type Deps struct {
	Client   CommandSink
	Deployer *agent.DockerDeployer
	State    *State
	// ServerAddr is the agent-facing server address (gateway).
	ServerAddr string
	// Settings provides cloud configuration (Yandex credentials, binary URL, etc.).
	Settings *types.ServerSettings
	// MonitoringURL is the vmauth base URL for metrics/logs ingestion.
	MonitoringURL string
	// MonitoringToken is the bearer token for vmauth.
	MonitoringToken string
	// AccountID is the per-tenant VictoriaMetrics account ID for data isolation.
	AccountID int32
	// JWTIssuer issues tokens for agent authentication.
	JWTIssuer *auth.JWTIssuer
	// TenantID is the tenant running this job.
	TenantID string
}

// task is one recipe/infra step. Replaces the old dag.Task now that there is no
// graph executor — steps run sequentially. noopTask (managed-YDB placeholder)
// lives in task_patroni.go.
type task interface {
	Execute(nc *NodeContext) error
}

// Deploy provisions the network + machines for real (docker containers /
// terraform VMs) and populates deps.State with the resulting targets, endpoint,
// and teardown handles.
func Deploy(nc *NodeContext, cfg types.RunConfig, deps Deps) error {
	net := &networkTask{cfg: cfg.Network, provider: cfg.Provider, deployer: deps.Deployer, state: deps.State, runID: cfg.ID}
	if err := net.Execute(nc); err != nil {
		return fmt.Errorf("deploy: network: %w", err)
	}
	machines := &machinesTask{runCfg: cfg, state: deps.State, deployer: deps.Deployer, serverAddr: deps.ServerAddr, settings: deps.Settings, jwtIssuer: deps.JWTIssuer, tenantID: deps.TenantID}
	if err := machines.Execute(nc); err != nil {
		return fmt.Errorf("deploy: machines: %w", err)
	}
	return nil
}

// Teardown destroys the run's infrastructure from the State handles.
func Teardown(nc *NodeContext, cfg types.RunConfig, deps Deps) error {
	td := &teardownTask{provider: cfg.Provider, state: deps.State, deployer: deps.Deployer, settings: deps.Settings}
	return td.Execute(nc)
}

// BuildRecipe runs the command-building tasks sequentially in dependency order,
// collecting their per-target command sequences into deps.Client (the plan
// sink). No agent I/O happens here — the collected commands are executed later
// as Temporal activities. deps.State must already hold the deployed targets +
// endpoint (from Deploy).
func BuildRecipe(nc *NodeContext, cfg types.RunConfig, deps Deps) error {
	b := &builder{cfg: cfg, deps: deps}
	return b.recipe(nc)
}

type builder struct {
	cfg  types.RunConfig
	deps Deps
}

func (b *builder) run(nc *NodeContext, label string, t task) error {
	if err := t.Execute(nc); err != nil {
		return fmt.Errorf("recipe: %s: %w", label, err)
	}
	return nil
}

func (b *builder) recipe(nc *NodeContext) error {
	d := b.deps
	cfg := b.cfg

	// Bring-your-own database: only bootstrap + install/run stroppy against the
	// supplied endpoint (already set on State by the caller for external runs).
	if cfg.ExternalDB != nil {
		host, port := splitHostPort(cfg.ExternalDB.Endpoint)
		if host == "" {
			return fmt.Errorf("external_db.endpoint must be host:port")
		}
		d.State.SetDBEndpoint(host, port)
		if err := b.run(nc, "bootstrap", &bootstrapTask{client: d.Client, state: d.State}); err != nil {
			return err
		}
		if err := b.run(nc, "install_stroppy", &stroppyInstallTask{client: d.Client, state: d.State, stroppy: cfg.Stroppy}); err != nil {
			return err
		}
		return b.run(nc, "run_stroppy", b.stroppyRunTask())
	}

	// 1. bootstrap base packages on every machine.
	if err := b.run(nc, "bootstrap", &bootstrapTask{client: d.Client, state: d.State}); err != nil {
		return err
	}

	// 2. etcd (postgres HA with etcd) — before DB configure.
	if b.needsEtcd() {
		if err := b.run(nc, "install_etcd", &etcdInstallTask{client: d.Client, state: d.State}); err != nil {
			return err
		}
		if err := b.run(nc, "configure_etcd", &etcdConfigTask{client: d.Client, state: d.State}); err != nil {
			return err
		}
	}

	// 3. install DB.
	installDB, configDB, err := b.dbTasks()
	if err != nil {
		return err
	}
	if err := b.run(nc, "install_db", installDB); err != nil {
		return err
	}

	// 4. configure DB (Patroni replaces the plain configure phase entirely).
	if b.needsPatroni() {
		if err := b.run(nc, "install_patroni", &patroniInstallTask{client: d.Client, state: d.State}); err != nil {
			return err
		}
		if err := b.run(nc, "configure_patroni", &patroniConfigTask{client: d.Client, state: d.State, version: cfg.Database.Version, topology: cfg.Database.Postgres, overrides: cfg.Database.RenderedConfigOverrides}); err != nil {
			return err
		}
	} else {
		if err := b.run(nc, "configure_db", configDB); err != nil {
			return err
		}
	}

	// 5. YDB cluster init + dynamic node start (must run before monitor config).
	if b.needsYDBInit() {
		if err := b.run(nc, "init_ydb_cluster", &ydbInitTask{client: d.Client, state: d.State, topology: cfg.Database.YDB}); err != nil {
			return err
		}
		if err := b.run(nc, "start_ydb_database", &ydbStartDBTask{client: d.Client, state: d.State, topology: cfg.Database.YDB, overrides: cfg.Database.RenderedConfigOverrides, pgwirePort: ydbPgwirePort(cfg)}); err != nil {
			return err
		}
	}

	// 6. CockroachDB cluster init (one-shot on the first node).
	if b.needsCockroachInit() {
		if err := b.run(nc, "init_cockroach", &cockroachInitTask{client: d.Client, state: d.State, topology: cfg.Database.Cockroach}); err != nil {
			return err
		}
	}

	// 7. monitoring install + configure (config after DB is up so exporters connect).
	if err := b.run(nc, "install_monitor", &monitorInstallTask{client: d.Client, state: d.State, dbKind: cfg.Database.Kind, serverAddr: d.ServerAddr}); err != nil {
		return err
	}
	if err := b.run(nc, "configure_monitor", &monitorConfigTask{client: d.Client, state: d.State, monitor: cfg.Monitor, runID: cfg.ID, dbKind: cfg.Database.Kind, ydbCombined: isYDBCombined(cfg.Database), monitoringURL: d.MonitoringURL, monitoringToken: d.MonitoringToken, accountID: d.AccountID}); err != nil {
		return err
	}

	// 8. pgbouncer (postgres HA, colocated on DB nodes).
	if b.needsPgBouncer() {
		if err := b.run(nc, "install_pgbouncer", &pgBouncerInstallTask{client: d.Client, state: d.State}); err != nil {
			return err
		}
		if err := b.run(nc, "configure_pgbouncer", &pgBouncerConfigTask{client: d.Client, state: d.State, topology: cfg.Database.Postgres, overrides: cfg.Database.RenderedConfigOverrides}); err != nil {
			return err
		}
	}

	// 9. proxy (HAProxy for PG/Picodata/YDB, ProxySQL for MySQL/MariaDB).
	if b.needsProxy() {
		if err := b.run(nc, "install_proxy", &proxyInstallTask{client: d.Client, state: d.State, dbKind: cfg.Database.Kind}); err != nil {
			return err
		}
		mysqlT := cfg.Database.MySQL
		if cfg.Database.Kind == types.DatabaseMariaDB {
			mysqlT = cfg.Database.MariaDB
		}
		if err := b.run(nc, "configure_proxy", &proxyConfigTask{client: d.Client, state: d.State, dbKind: cfg.Database.Kind,
			pgTopology: cfg.Database.Postgres, mysqlTopology: mysqlT, picoTopology: cfg.Database.Picodata, ydbTopology: cfg.Database.YDB,
			overrides: cfg.Database.RenderedConfigOverrides}); err != nil {
			return err
		}
	}

	// 10. stroppy install + run.
	if err := b.run(nc, "install_stroppy", &stroppyInstallTask{client: d.Client, state: d.State, stroppy: cfg.Stroppy}); err != nil {
		return err
	}
	return b.run(nc, "run_stroppy", b.stroppyRunTask())
}

func (b *builder) stroppyRunTask() *stroppyRunTask {
	cfg := b.cfg
	d := b.deps
	ss := types.DefaultStroppySettings()
	// Metric prefix = runID so each run's metrics are namespaced. PromQL metric
	// names don't support dashes.
	ss.OTLPMetricPrefix = strings.ReplaceAll(cfg.ID, "-", "_") + "_"
	if d.MonitoringURL != "" {
		ss.SetFromMonitoringURL(d.MonitoringURL, d.MonitoringToken, d.AccountID)
	}
	return &stroppyRunTask{
		client:          d.Client,
		state:           d.State,
		stroppy:         cfg.Stroppy,
		stroppySettings: ss,
		dbKind:          cfg.Database.Kind,
		dbCfg:           cfg.Database,
		runID:           cfg.ID,
		monitoringURL:   d.MonitoringURL,
		monitoringToken: d.MonitoringToken,
		accountID:       d.AccountID,
	}
}

func splitHostPort(addr string) (string, int) {
	idx := strings.LastIndex(addr, ":")
	if idx <= 0 || idx == len(addr)-1 {
		return "", 0
	}
	host := addr[:idx]
	portStr := addr[idx+1:]
	port := 0
	for _, c := range portStr {
		if c < '0' || c > '9' {
			return "", 0
		}
		port = port*10 + int(c-'0')
	}
	return host, port
}

// --- conditional helpers ---

func (b *builder) needsEtcd() bool {
	return b.cfg.Database.Kind == types.DatabasePostgres && b.cfg.Database.Postgres != nil && b.cfg.Database.Postgres.Etcd
}

func (b *builder) needsPatroni() bool {
	return b.cfg.Database.Kind == types.DatabasePostgres && b.cfg.Database.Postgres != nil && b.cfg.Database.Postgres.Patroni
}

func (b *builder) needsPgBouncer() bool {
	return b.cfg.Database.Kind == types.DatabasePostgres && b.cfg.Database.Postgres != nil && b.cfg.Database.Postgres.PgBouncer
}

func (b *builder) needsProxy() bool {
	db := b.cfg.Database
	switch db.Kind {
	case types.DatabasePostgres:
		return db.Postgres != nil && db.Postgres.HAProxy != nil
	case types.DatabaseMySQL:
		return db.MySQL != nil && db.MySQL.ProxySQL != nil
	case types.DatabaseMariaDB:
		return db.MariaDB != nil && db.MariaDB.ProxySQL != nil
	case types.DatabasePicodata:
		return db.Picodata != nil && db.Picodata.HAProxy != nil
	case types.DatabaseYDB:
		return db.YDB != nil && db.YDB.HAProxy != nil
	}
	return false
}

// ydbPgwirePort returns the port ydbd exposes its postgres-wire surface on, or 0
// when the run isn't using the ydb-pgwire protocol.
func ydbPgwirePort(cfg types.RunConfig) int {
	if cfg.Stroppy.Protocol != types.ProtocolYDBPgwire {
		return 0
	}
	return types.Protocols[types.ProtocolYDBPgwire].Port
}

func (b *builder) needsYDBInit() bool {
	return b.cfg.Database.Kind == types.DatabaseYDB && b.cfg.Database.YDB != nil
}

func (b *builder) needsCockroachInit() bool {
	return b.cfg.Database.Kind == types.DatabaseCockroach && b.cfg.Database.Cockroach != nil
}

// dbTasks returns the install + configure tasks for the configured engine.
func (b *builder) dbTasks() (install task, config task, err error) {
	db := b.cfg.Database
	pkg := b.cfg.ResolvedPackage
	d := b.deps
	switch db.Kind {
	case types.DatabasePostgres:
		return &pgInstallTask{client: d.Client, state: d.State, version: db.Version, topology: db.Postgres, pkg: pkg},
			&pgConfigTask{client: d.Client, state: d.State, version: db.Version, topology: db.Postgres, overrides: db.RenderedConfigOverrides}, nil
	case types.DatabaseMySQL:
		return &mysqlInstallTask{client: d.Client, state: d.State, version: db.Version, topology: db.MySQL, pkg: pkg},
			&mysqlConfigTask{client: d.Client, state: d.State, topology: db.MySQL, overrides: db.RenderedConfigOverrides}, nil
	case types.DatabaseMariaDB:
		return &mysqlInstallTask{client: d.Client, state: d.State, version: db.Version, topology: db.MariaDB, pkg: pkg},
			&mysqlConfigTask{client: d.Client, state: d.State, topology: db.MariaDB, overrides: db.RenderedConfigOverrides}, nil
	case types.DatabasePicodata:
		return &picoInstallTask{client: d.Client, state: d.State, version: db.Version, topology: db.Picodata, pkg: pkg},
			&picoConfigTask{client: d.Client, state: d.State, topology: db.Picodata, overrides: db.RenderedConfigOverrides}, nil
	case types.DatabaseYDB:
		return &ydbInstallTask{client: d.Client, state: d.State, version: db.Version, topology: db.YDB, pkg: pkg},
			&ydbConfigTask{client: d.Client, state: d.State, topology: db.YDB, overrides: db.RenderedConfigOverrides, pgwirePort: ydbPgwirePort(b.cfg)}, nil
	case types.DatabaseYDBManaged:
		return &noopTask{}, &noopTask{}, nil
	case types.DatabaseCockroach:
		if db.Cockroach == nil {
			return nil, nil, fmt.Errorf("cockroach topology missing for database.kind=cockroach")
		}
		return &cockroachInstallTask{client: d.Client, state: d.State, version: db.Version},
			&cockroachConfigTask{client: d.Client, state: d.State, topology: db.Cockroach}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported database kind %q", db.Kind)
	}
}
