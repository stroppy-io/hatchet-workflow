package postgres

import (
	"fmt"
	"sort"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbcredentials"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

const (
	// postgresReplicationUser is the dedicated least-privilege role used for
	// streaming replication (REPLICATION LOGIN only, no superuser).
	postgresReplicationUser = "replicator"
	// postgresReplicationPassword is the default replication password for these
	// ephemeral benchmark clusters. It is written only into the replica's
	// .pgpass (mode 0600) and used to create the replicator role on the master.
	// Override via params/secrets for hardened, long-lived deployments.
	postgresReplicationPassword = "stroppy_replication"
)

type DeploymentRenderer struct{}

func (r DeploymentRenderer) Supports(component *topologypb.Component) bool {
	return component != nil && component.GetEngine() == postgresEngine
}

func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	spec, ok := postgresDeploymentSpecs[ctx.Component.GetRole()]
	if !ok {
		return nil, fmt.Errorf("unsupported postgres component role %q", ctx.Component.GetRole())
	}
	wiring, err := postgresResolveWiring(ctx)
	if err != nil {
		return nil, err
	}
	ec := postgresEngineComponent(ctx.Component, ctx.Database, ctx.RenderOverrides, ctx.DatabasePackage, postgresDependencies(ctx), spec, wiring)
	return deploymentbuilder.RenderComponentDeployment(ctx, ec), nil
}

func (r DeploymentRenderer) RenderPreview(ctx deploymentbuilder.PreviewContext) ([]*deploymentpb.RenderArtifact, error) {
	spec, ok := postgresDeploymentSpecs[ctx.Component.GetRole()]
	if !ok {
		return nil, fmt.Errorf("unsupported postgres component role %q", ctx.Component.GetRole())
	}
	// Preview has no runtime addresses; standby/backends/etcd-cluster render empty.
	ec := postgresEngineComponent(ctx.Component, ctx.Database, ctx.RenderOverrides, ctx.DatabasePackage, postgresPreviewDependencies(ctx), spec, postgresWiring{})
	return deploymentbuilder.RenderComponentPreview(ctx, ec), nil
}

// postgresWiring carries runtime-resolved cluster topology for one component.
type postgresWiring struct {
	masterAddr  string     // master host:port (replica streaming source)
	backends    []pgServer // haproxy backend pg/pgbouncer endpoints
	etcdPeers   []etcdPeer // all etcd peers (etcd initial-cluster)
	etcdClients []string   // etcd client host:port (patroni dcs)
	ownHost     string     // this component's own private host (patroni connect_address)

	// patroniManaged is true when a patroni coordinator is colocated on this
	// node. Patroni then owns the postgres process entirely, so the plain
	// master/replica systemd unit must NOT also start postgres (port + data_dir
	// conflict). See https://patroni.readthedocs.io/ — Patroni must be the only
	// supervisor of the managed PostgreSQL instance.
	patroniManaged bool
	// patroniCluster is true when the topology runs Patroni at all; haproxy then
	// health-checks the patroni REST API (GET /primary) instead of a blind TCP
	// connect so it routes writes only to the current leader.
	patroniCluster bool
}

type pgServer struct {
	name string
	addr string
}

type etcdPeer struct {
	name string // component id
	addr string // peer host:port
	self bool
}

func postgresResolveWiring(ctx deploymentbuilder.RenderContext) (postgresWiring, error) {
	wiring := postgresWiring{
		patroniManaged: nodeHasPatroni(ctx),
		patroniCluster: topologyHasPatroni(ctx),
	}
	if own, ok := deploymentbuilder.OwnPrivateEndpoint(ctx); ok {
		wiring.ownHost = own.Address
	}

	switch ctx.Component.GetRole() {
	case postgresRoleReplica:
		masters, err := deploymentbuilder.DependencyTargets(ctx, func(_ *topologypb.Connection, target *topologypb.Component) bool {
			return target.GetRole() == postgresRoleMaster
		})
		if err != nil {
			return postgresWiring{}, err
		}
		if len(masters) > 0 {
			wiring.masterAddr = masters[0].AddressPort()
		}
	case postgresRoleHaproxy:
		backends, err := deploymentbuilder.DependencyTargets(ctx, nil)
		if err != nil {
			return postgresWiring{}, err
		}
		for i, backend := range backends {
			wiring.backends = append(wiring.backends, pgServer{
				name: fmt.Sprintf("pg-%d", i+1),
				addr: backend.AddressPort(),
			})
		}
	case postgresRoleEtcd:
		peers, err := deploymentbuilder.ComponentTargets(ctx, isPostgresEtcd, "etcd_peer", 2380)
		if err != nil {
			return postgresWiring{}, err
		}
		for _, peer := range peers {
			wiring.etcdPeers = append(wiring.etcdPeers, etcdPeer{
				name: peer.ComponentID,
				addr: peer.AddressPort(),
				self: peer.ComponentID == ctx.Component.GetId(),
			})
		}
	case postgresRolePatroni:
		clients, err := deploymentbuilder.ComponentTargets(ctx, isPostgresEtcd, "etcd_client", 2379)
		if err != nil {
			return postgresWiring{}, err
		}
		for _, client := range clients {
			wiring.etcdClients = append(wiring.etcdClients, client.AddressPort())
		}
	}
	return wiring, nil
}

func isPostgresEtcd(c *topologypb.Component) bool {
	return c.GetEngine() == postgresEngine && c.GetRole() == postgresRoleEtcd
}

// nodeHasPatroni reports whether a patroni coordinator is colocated on the same
// node as the component being rendered. When true, Patroni owns postgres and the
// plain master/replica unit must stay passive.
func nodeHasPatroni(ctx deploymentbuilder.RenderContext) bool {
	for _, id := range ctx.Node.GetComponentIds() {
		if c, ok := ctx.Topology.Component(id); ok && c.GetRole() == postgresRolePatroni {
			return true
		}
	}
	return false
}

// topologyHasPatroni reports whether any patroni coordinator exists in the whole
// cluster, used by haproxy to decide between a leader-aware REST health check and
// a plain TCP connect check.
func topologyHasPatroni(ctx deploymentbuilder.RenderContext) bool {
	for _, c := range ctx.Topology.Spec().GetComponents() {
		if c.GetRole() == postgresRolePatroni {
			return true
		}
	}
	return false
}

func postgresEngineComponent(
	component *topologypb.Component,
	database *domain.Database,
	overrides *deploymentpb.RenderOverrideSet,
	dbPackage *domain.Package,
	dependencies []string,
	spec postgresDeploymentSpec,
	wiring postgresWiring,
) deploymentbuilder.EngineComponent {
	configFile, configOrigin := postgresEffectiveConfigFile(component, database, overrides, wiring)
	configDir := deploymentbuilder.ConfigDir(component.GetId())

	ec := deploymentbuilder.EngineComponent{
		Engine:            postgresEngine,
		GlobalPriority:    spec.globalPriority,
		NodePriority:      spec.nodePriority,
		Dependencies:      dependencies,
		ConfigArtifactID:  configArtifactID(component.GetId(), component.GetRole()),
		ConfigFile:        configFile,
		ConfigOrigin:      configOrigin,
		DefaultConfigFile: postgresDefaultConfigFile(component, database, postgresWiring{}),
		InstallCommands:   postgresInstallCommands(component, dbPackage),
		ServiceFile:       postgresServiceFile(component.GetId(), component.GetRole(), configDir, wiring),
		Healthcheck:       postgresHealthcheckCommand(component, wiring),
	}
	if hbaFile := postgresHBAFile(component.GetId(), component.GetRole()); hbaFile != nil {
		ec.ExtraFiles = append(ec.ExtraFiles, deploymentbuilder.EngineFile{
			StepID:        "040_write_hba",
			StepOrder:     40,
			ArtifactName:  "pg_hba.conf",
			ArtifactLabel: "hba",
			LockReason:    "pg_hba.conf is generated by the deployment renderer",
			File:          hbaFile,
		})
	}
	switch component.GetRole() {
	case postgresRoleMaster:
		ec.ExtraFiles = append(ec.ExtraFiles, deploymentbuilder.EngineFile{
			StepID:        "050_write_replication_setup",
			StepOrder:     50,
			ArtifactName:  "replication-setup.sql",
			ArtifactLabel: "replication_setup",
			File:          postgresReplicationSetupFile(component.GetId()),
		})
	case postgresRoleReplica:
		if wiring.masterAddr != "" {
			ec.ExtraFiles = append(ec.ExtraFiles, deploymentbuilder.EngineFile{
				StepID:        "045_write_pgpass",
				StepOrder:     45,
				ArtifactName:  ".pgpass",
				ArtifactLabel: "pgpass",
				File:          postgresPgpassFile(component.GetId()),
			})
		}
	case postgresRolePgbouncer:
		ec.ExtraFiles = append(ec.ExtraFiles, deploymentbuilder.EngineFile{
			StepID:        "045_write_users",
			StepOrder:     45,
			ArtifactName:  "users.txt",
			ArtifactLabel: "pgbouncer_users",
			File: &common.File{
				Info: &common.File_Info{
					Path:          deploymentbuilder.ConfigDir(component.GetId()) + "/users.txt",
					Mode:          0644,
					CreateParents: true,
				},
				Content: &common.File_Text{Text: `"postgres" ""` + "\n"},
			},
		})
	}
	return ec
}

type postgresDeploymentSpec struct {
	globalPriority uint32
	nodePriority   uint32
}

var postgresDeploymentSpecs = map[string]postgresDeploymentSpec{
	postgresRoleEtcd:      {globalPriority: 10, nodePriority: 10},
	postgresRoleMaster:    {globalPriority: 20, nodePriority: 20},
	postgresRoleReplica:   {globalPriority: 30, nodePriority: 20},
	postgresRolePatroni:   {globalPriority: 40, nodePriority: 30},
	postgresRolePgbouncer: {globalPriority: 50, nodePriority: 40},
	postgresRoleHaproxy:   {globalPriority: 60, nodePriority: 50},
}

func postgresDependencies(ctx deploymentbuilder.RenderContext) []string {
	dependencies := deploymentbuilder.DependencyIDs(ctx, func(target *topologypb.Component) bool {
		return ctx.Component.GetRole() != postgresRoleEtcd || target.GetRole() != postgresRoleEtcd
	})
	sort.Strings(dependencies)
	return dependencies
}

func postgresPreviewDependencies(ctx deploymentbuilder.PreviewContext) []string {
	return postgresDependencies(deploymentbuilder.RenderContext{
		Topology:  ctx.Topology,
		Component: ctx.Component,
		Node:      ctx.Node,
	})
}

func postgresInstallCommands(component *topologypb.Component, dbPackage *domain.Package) []string {
	switch component.GetRole() {
	case postgresRoleMaster, postgresRoleReplica:
		var commands []string
		if dbPackage == nil {
			commands = []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y postgresql postgresql-contrib"}
		} else {
			commands = append([]string{}, dbPackage.GetPreInstall()...)
			if len(dbPackage.GetAptPackages()) > 0 {
				commands = append(commands, "DEBIAN_FRONTEND=noninteractive apt-get install -y "+strings.Join(dbPackage.GetAptPackages(), " "))
			}
			if dbPackage.GetDebFilename() != "" {
				commands = append(commands, deploymentbuilder.InstallDebFilenameCommand(dbPackage.GetDebFilename(), "/tmp/stroppy-postgres-package.deb"))
			}
		}
		commands = append(commands, "systemctl disable --now postgresql || true")
		if len(commands) == 0 {
			commands = append(commands, "true")
		}
		return commands
	case postgresRoleHaproxy:
		// postgresql-client provides pg_isready (healthcheck proves haproxy can
		// route to a live primary); socat queries the haproxy stats socket to
		// surface the per-backend check verdict when it does not.
		return []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y haproxy postgresql-client socat"}
	case postgresRolePgbouncer:
		return []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y pgbouncer"}
	case postgresRolePatroni:
		// python3-etcd is the client Patroni's etcd3 DCS backend speaks through;
		// the patroni package's OR-dependency can otherwise resolve to a non-etcd
		// client and Patroni then fails to find a usable DCS at startup.
		return []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y patroni python3-etcd"}
	case postgresRoleEtcd:
		// The distro etcd package auto-starts its own etcd.service bound to the
		// default 2379/2380 ports; left running it steals the ports from our
		// stroppy etcd unit, which then crashloops and fails the healthcheck.
		// Install (etcd, or the split etcd-server/-client on newer Ubuntu) and
		// disable the packaged service, mirroring the postgresql handling above.
		return []string{
			"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y etcd || DEBIAN_FRONTEND=noninteractive apt-get install -y etcd-server etcd-client",
			"systemctl disable --now etcd 2>/dev/null || true",
		}
	default:
		return []string{"true"}
	}
}

func postgresServiceFile(componentID, role, configDir string, wiring postgresWiring) *common.File {
	return deploymentbuilder.EngineServiceFile(componentID, postgresServiceUnit(componentID, role, configDir, wiring))
}

func postgresServiceUnit(componentID, role, configDir string, wiring postgresWiring) string {
	cfgPath := configPath(componentID, role)
	// When Patroni manages this node, the postgres data instance is owned by the
	// colocated patroni unit. The master/replica unit becomes a passive marker so
	// it does not fight Patroni for port 5432 / the data directory.
	if wiring.patroniManaged && (role == postgresRoleMaster || role == postgresRoleReplica) {
		return postgresPassiveServiceUnit(componentID, role)
	}
	switch role {
	case postgresRoleMaster:
		return postgresMasterServiceUnit(componentID, role, configDir, cfgPath)
	case postgresRoleReplica:
		if wiring.masterAddr != "" {
			return postgresReplicaServiceUnit(componentID, role, configDir, cfgPath, wiring.masterAddr)
		}
		return postgresDatabaseServiceUnit(componentID, role, configDir, cfgPath)
	case postgresRoleHaproxy:
		// -W (master-worker, foreground) not -Ws: -Ws also enables sd_notify which
		// requires a Type=notify unit. Under the Type=simple unit here, -Ws left
		// the unit reported as not-active even though the worker served traffic,
		// so the healthcheck's `systemctl is-active` short-circuited and the gate
		// never ran. -W keeps the master in the foreground for Type=simple.
		return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, "/usr/sbin/haproxy -W -f "+deploymentbuilder.ShellQuote(cfgPath))
	case postgresRolePgbouncer:
		return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, "/usr/sbin/runuser -u postgres -- /usr/sbin/pgbouncer "+deploymentbuilder.ShellQuote(cfgPath))
	case postgresRolePatroni:
		return postgresPatroniServiceUnit(componentID, role, configDir, cfgPath)
	case postgresRoleEtcd:
		return postgresEtcdServiceUnit(componentID, role, configDir, cfgPath, wiring)
	default:
		return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, "/bin/false")
	}
}

func postgresDatabaseServiceUnit(componentID, role, configDir, configPath string) string {
	dataDir := postgresDataDir(componentID)
	runDir := postgresRunDir(componentID)
	hbaPath := postgresHBAPath(componentID)
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud PostgreSQL %s component %s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=%s/topology.env
ExecStartPre=/bin/install -d -m 0755 %s %s
ExecStartPre=/bin/mkdir -p %s %s
ExecStartPre=/bin/chown -R postgres:postgres %s %s
ExecStartPre=/bin/sh -ec "test -s %s/PG_VERSION || /usr/sbin/runuser -u postgres -- \"$$(find /usr/lib/postgresql -path '*/bin/initdb' | sort -V | tail -n1)\" -D %s"
ExecStart=/bin/sh -ec "exec /usr/sbin/runuser -u postgres -- \"$$(find /usr/lib/postgresql -path '*/bin/postgres' | sort -V | tail -n1)\" -D %s -c config_file=%s -c hba_file=%s -c listen_addresses='*' -c port=5432 -c unix_socket_directories=%s"
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`,
		role,
		componentID,
		configDir,
		deploymentbuilder.ShellQuote(postgresDataRoot()),
		deploymentbuilder.ShellQuote(postgresRunRoot()),
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(runDir),
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(runDir),
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(configPath),
		deploymentbuilder.ShellQuote(hbaPath),
		deploymentbuilder.ShellQuote(runDir),
	)
}

// postgresMasterServiceUnit is the master database unit plus an ExecStartPost
// that provisions the dedicated replication role (idempotent) once the server
// is ready, so standbys can authenticate with scram instead of trust.
func postgresMasterServiceUnit(componentID, role, configDir, cfgPath string) string {
	base := strings.TrimSuffix(postgresDatabaseServiceUnit(componentID, role, configDir, cfgPath), `
[Install]
WantedBy=multi-user.target
`)
	setupPath := configDir + "/replication-setup.sql"
	return base + fmt.Sprintf(`ExecStartPost=/bin/sh -ec "for i in $$(seq 1 30); do if pg_isready -U postgres -h 127.0.0.1 -p 5432; then exec /usr/sbin/runuser -u postgres -- psql -v ON_ERROR_STOP=1 -U postgres -h 127.0.0.1 -p 5432 < %s; fi; sleep 1; done; exit 1"

[Install]
WantedBy=multi-user.target
`, deploymentbuilder.ShellQuote(setupPath))
}

// postgresReplicaServiceUnit bootstraps a streaming standby: an empty data dir
// is filled with pg_basebackup from the master as the dedicated replication
// user (-R writes standby.signal and primary_conninfo), then the same postgres
// daemon starts as a hot standby. The replication password is read from a
// 0600 .pgpass via PGPASSFILE — never embedded in the unit or config.
func postgresReplicaServiceUnit(componentID, role, configDir, configPath, masterAddr string) string {
	dataDir := postgresDataDir(componentID)
	runDir := postgresRunDir(componentID)
	hbaPath := postgresHBAPath(componentID)
	pgpassPath := postgresPgpassPath(componentID)
	host, port := postgresHostPort(masterAddr)
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud PostgreSQL %s component %s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=%s/topology.env
Environment=PGPASSFILE=%s
ExecStartPre=/bin/install -d -m 0755 %s %s
ExecStartPre=/bin/mkdir -p %s %s
ExecStartPre=/bin/chown -R postgres:postgres %s %s
ExecStartPre=/bin/chown postgres:postgres %s
ExecStartPre=/bin/chmod 0600 %s
ExecStartPre=/bin/sh -ec "test -s %s/PG_VERSION || for i in $$(seq 1 30); do /usr/sbin/runuser -u postgres -- env PGPASSFILE=%s \"$$(find /usr/lib/postgresql -path '*/bin/pg_basebackup' | sort -V | tail -n1)\" -h %s -p %d -U %s -w -D %s -R && break; sleep 3; done; test -s %s/PG_VERSION && chmod 0700 %s"
ExecStart=/bin/sh -ec "exec /usr/sbin/runuser -u postgres -- \"$$(find /usr/lib/postgresql -path '*/bin/postgres' | sort -V | tail -n1)\" -D %s -c config_file=%s -c hba_file=%s -c listen_addresses='*' -c port=5432 -c unix_socket_directories=%s"
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`,
		role,
		componentID,
		configDir,
		deploymentbuilder.ShellQuote(pgpassPath),
		deploymentbuilder.ShellQuote(postgresDataRoot()),
		deploymentbuilder.ShellQuote(postgresRunRoot()),
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(runDir),
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(runDir),
		deploymentbuilder.ShellQuote(pgpassPath),
		deploymentbuilder.ShellQuote(pgpassPath),
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(pgpassPath),
		host,
		port,
		deploymentbuilder.ShellQuote(postgresReplicationUser),
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(configPath),
		deploymentbuilder.ShellQuote(hbaPath),
		deploymentbuilder.ShellQuote(runDir),
	)
}

func postgresPgpassPath(componentID string) string {
	return deploymentbuilder.ConfigDir(componentID) + "/.pgpass"
}

// postgresPgpassFile holds the replication credential for pg_basebackup and the
// running standby (host:port:db:user:password), mode 0600.
func postgresPgpassFile(componentID string) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          postgresPgpassPath(componentID),
			Mode:          0600,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: fmt.Sprintf("*:*:*:%s:%s\n", postgresReplicationUser, postgresReplicationPassword)},
	}
}

// postgresReplicationSetupFile creates the least-privilege replication role on
// the master if it does not already exist.
func postgresReplicationSetupFile(componentID string) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          deploymentbuilder.ConfigDir(componentID) + "/replication-setup.sql",
			Mode:          0640,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: fmt.Sprintf(`DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%s') THEN
    EXECUTE format('CREATE ROLE %%I WITH REPLICATION LOGIN PASSWORD %%L', '%s', '%s');
  END IF;
END;
$$;
SET password_encryption = 'scram-sha-256';
ALTER ROLE %s WITH LOGIN PASSWORD '%s';
`, postgresReplicationUser, postgresReplicationUser, postgresReplicationPassword, dbcredentials.PostgresUser, dbcredentials.PostgresPassword)},
	}
}

// postgresEtcdServiceUnit starts etcd with a resolved static initial-cluster.
// Without peers (preview) it falls back to a single-node bootstrap.
func postgresEtcdServiceUnit(componentID, role, configDir, cfgPath string, wiring postgresWiring) string {
	if len(wiring.etcdPeers) == 0 {
		return postgresEtcdUnit(componentID, role, configDir, "/usr/bin/etcd --config-file "+deploymentbuilder.ShellQuote(cfgPath))
	}

	var ownAddr string
	clusterParts := make([]string, 0, len(wiring.etcdPeers))
	for _, peer := range wiring.etcdPeers {
		host, _ := postgresHostPort(peer.addr)
		clusterParts = append(clusterParts, fmt.Sprintf("%s=http://%s:2380", peer.name, host))
		if peer.self {
			ownAddr = host
		}
	}
	cluster := strings.Join(clusterParts, ",")
	execStart := fmt.Sprintf("/usr/bin/etcd --name %s --data-dir %s --initial-cluster %s --initial-advertise-peer-urls http://%s:2380 --listen-peer-urls http://0.0.0.0:2380 --advertise-client-urls http://%s:2379 --listen-client-urls http://0.0.0.0:2379",
		deploymentbuilder.ShellQuote(componentID),
		deploymentbuilder.ShellQuote(postgresDataDir(componentID)),
		deploymentbuilder.ShellQuote(cluster),
		ownAddr,
		ownAddr,
	)
	return postgresEtcdUnit(componentID, role, configDir, execStart)
}

func postgresEtcdUnit(componentID, role, configDir, execStart string) string {
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud %s component %s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=%s/topology.env
ExecStartPre=/bin/install -d -m 0755 %s
ExecStart=%s
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`,
		role,
		componentID,
		configDir,
		deploymentbuilder.ShellQuote(postgresDataRoot()),
		execStart,
	)
}

// postgresPassiveServiceUnit is a no-op marker unit used for master/replica
// components when Patroni owns the postgres instance on the node. It stays
// "active" so dependency/healthcheck gating passes, but it never touches port
// 5432 or the data directory.
func postgresPassiveServiceUnit(componentID, role string) string {
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud PostgreSQL %s component %s (managed by Patroni)
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/true

[Install]
WantedBy=multi-user.target
`, role, componentID)
}

// postgresPatroniServiceUnit runs Patroni as the postgres user. The PostgreSQL
// bin_dir is resolved at start so the unit works across Ubuntu releases (PG 14
// on 22.04, PG 16 on 24.04, ...) and exported via PATRONI_POSTGRESQL_BIN_DIR,
// which overrides the YAML value.
func postgresPatroniServiceUnit(componentID, role, configDir, cfgPath string) string {
	baseID := strings.TrimSuffix(componentID, "-"+postgresRolePatroni)
	dataDir := postgresDataDir(baseID)
	runDir := postgresRunDir(baseID)
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud PostgreSQL %s component %s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=%s/topology.env
ExecStartPre=/bin/install -d -m 0700 -o postgres -g postgres %s
ExecStartPre=/bin/install -d -m 0755 -o postgres -g postgres %s
ExecStart=/bin/sh -ec "BIN=$$(dirname \"$$(find /usr/lib/postgresql -path '*/bin/postgres' | sort -V | tail -n1)\"); exec /usr/sbin/runuser -u postgres -- env PATRONI_POSTGRESQL_BIN_DIR=\"$$BIN\" /usr/bin/patroni %s"
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`,
		role,
		componentID,
		configDir,
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(runDir),
		deploymentbuilder.ShellQuote(cfgPath),
	)
}

func postgresHBAFile(componentID, role string) *common.File {
	switch role {
	case postgresRoleMaster, postgresRoleReplica:
		return &common.File{
			Info: &common.File_Info{
				Path:          postgresHBAPath(componentID),
				Mode:          0644,
				CreateParents: true,
			},
			Content: &common.File_Text{Text: `local all all trust
host all all 127.0.0.1/32 trust
host all all ::1/128 trust
host all all 0.0.0.0/0 scram-sha-256
host replication ` + postgresReplicationUser + ` 10.0.0.0/8 scram-sha-256
host replication ` + postgresReplicationUser + ` 172.16.0.0/12 scram-sha-256
host replication ` + postgresReplicationUser + ` 192.168.0.0/16 scram-sha-256
`},
		}
	default:
		return nil
	}
}

func postgresHealthcheckCommand(component *topologypb.Component, wiring postgresWiring) string {
	service := deploymentbuilder.ShellQuote(deploymentbuilder.ServiceName(component.GetId()))
	wrapRetry := func(cmd string) string {
		return "for i in $(seq 1 60); do if " + cmd + "; then exit 0; fi; sleep 3; done; exit 1"
	}
	switch component.GetRole() {
	case postgresRoleMaster, postgresRoleReplica:
		// Under Patroni the postgres instance is owned by the colocated patroni
		// unit (started in a later wave), so the passive marker is only checked
		// for liveness; the patroni component verifies postgres is serving.
		if wiring.patroniManaged {
			return wrapRetry("systemctl is-active --quiet " + service)
		}
		return wrapRetry("systemctl is-active --quiet " + service + " && pg_isready -h 127.0.0.1 -p 5432")
	case postgresRolePgbouncer:
		return wrapRetry("systemctl is-active --quiet " + service + " && pg_isready -h 127.0.0.1 -p 6432")
	case postgresRolePatroni:
		// Patroni REST /health returns 200 once the managed postgres is running.
		return wrapRetry("systemctl is-active --quiet " + service + " && curl -sf -o /dev/null http://127.0.0.1:8008/health")
	case postgresRoleHaproxy:
		// With Patroni, haproxy only forwards to the node whose patroni REST
		// reports a primary. Until a leader is elected and the L7 check marks it
		// up, haproxy has zero live servers and resets connections (the
		// "unexpected EOF" the workload otherwise hit). Gate on a real connection
		// through the frontend so the workload never starts before a primary is
		// routable; the retry budget covers leader election + check rise time.
		if wiring.patroniCluster {
			return postgresHaproxyPatroniHealthcheck(configPath(component.GetId(), postgresRoleHaproxy))
		}
		return wrapRetry("systemctl is-active --quiet " + service)
	default:
		return wrapRetry("systemctl is-active --quiet " + service)
	}
}

// postgresHaproxyPatroniHealthcheck gates the haproxy component on a real
// connection through its own frontend, and on failure dumps the patroni REST
// status (/, /primary, /leader) for every backend so a stuck cluster is
// diagnosable from the run logs without live VM access.
func postgresHaproxyPatroniHealthcheck(cfgPath string) string {
	cfg := deploymentbuilder.ShellQuote(cfgPath)
	// Gate on a full psql round-trip through the frontend (no `systemctl
	// is-active` precondition — a successful query already proves haproxy is up
	// and routing). pg_isready is avoided because its abrupt half-open probe can
	// read as "no response" through haproxy mode tcp even when the backend
	// serves. Allow up to ~6min: with synchronous_mode the patroni leader only
	// reports /primary=200 (the only state haproxy routes to) once a sync standby
	// has finished pg_basebackup and connected.
	return fmt.Sprintf(`for i in $(seq 1 120); do if PGCONNECT_TIMEOUT=5 psql 'host=127.0.0.1 port=5432 user=postgres dbname=postgres' -tAc 'select 1' >/dev/null 2>&1; then exit 0; fi; sleep 3; done; `+
		`echo "HAPROXY-DIAG: frontend not routable; probing patroni REST backends from %s"; `+
		`for ip in $(awk '/^[[:space:]]*server /{print $3}' %s | sed 's/:.*//'); do `+
		`echo "HAPROXY-DIAG host=$ip /primary=$(curl -s -m3 -o /dev/null -w '%%{http_code}' http://$ip:8008/primary) pg5432=$(pg_isready -h $ip -p 5432 -t 3 2>&1) tcp5432=$(timeout 3 bash -c "echo > /dev/tcp/$ip/5432" 2>&1 && echo open || echo closed)"; done; `+
		`echo "HAPROXY-DIAG stat:"; echo "show stat" | timeout 3 socat - /run/haproxy.sock 2>/dev/null | grep -E '^(ft_postgres|bk_postgres),' | sed 's/^/HAPROXY-DIAG-STAT /'; `+
		`echo "HAPROXY-DIAG psql-result: [$(PGCONNECT_TIMEOUT=5 psql 'host=127.0.0.1 port=5432 user=postgres dbname=postgres' -tAc 'select 1' 2>&1 | tr '\n' ' ')]"; `+
		`echo "HAPROXY-DIAG cfg:"; sed -n '/backend bk_postgres/,$p' %s; exit 1`,
		cfg, cfg, cfg)
}

func postgresDataDir(componentID string) string {
	return postgresDataRoot() + "/" + componentID
}

func postgresRunDir(componentID string) string {
	return postgresRunRoot() + "/" + componentID
}

func postgresDataRoot() string {
	return "/var/lib/stroppy-cloud"
}

func postgresRunRoot() string {
	return "/run/stroppy-cloud"
}

func postgresHBAPath(componentID string) string {
	return deploymentbuilder.ConfigDir(componentID) + "/pg_hba.conf"
}

// postgresHostPort splits a host:port pair, defaulting to the postgres port.
func postgresHostPort(addr string) (string, int) {
	idx := strings.LastIndex(addr, ":")
	if idx < 0 || strings.HasPrefix(addr, "[") {
		return addr, 5432
	}
	var port int
	if _, err := fmt.Sscanf(addr[idx+1:], "%d", &port); err != nil {
		return addr, 5432
	}
	return addr[:idx], port
}

func postgresEffectiveConfigFile(component *topologypb.Component, database *domain.Database, overrides *deploymentpb.RenderOverrideSet, wiring postgresWiring) (*common.File, deploymentpb.RenderArtifact_Origin) {
	artifactID := configArtifactID(component.GetId(), component.GetRole())
	if override, ok := deploymentbuilder.OverrideFile(overrides, component.GetId(), artifactID); ok && override.GetFile() != nil {
		return override.GetFile(), deploymentpb.RenderArtifact_ORIGIN_USER_OVERRIDE
	}
	return postgresDefaultConfigFile(component, database, wiring), deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT
}

func postgresDefaultConfigFile(component *topologypb.Component, database *domain.Database, wiring postgresWiring) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          configPath(component.GetId(), component.GetRole()),
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: postgresConfigContentForRole(component.GetId(), database, component.GetRole(), wiring)},
	}
}

func postgresConfigContentForRole(cid string, database *domain.Database, role string, wiring postgresWiring) string {
	options := postgresRoleOptions(database, role)
	switch role {
	case postgresRoleHaproxy:
		return postgresHaproxyConfig(options, wiring.backends, wiring.patroniCluster)
	case postgresRolePgbouncer:
		return postgresPgbouncerConfig(cid, options)
	case postgresRolePatroni:
		return postgresPatroniConfig(cid, database, options, wiring)
	default:
		return configContent(role, options)
	}
}

// postgresHaproxyConfig renders an haproxy.cfg fronting port 5432 across the
// resolved pg/pgbouncer backends. With Patroni the backend uses an HTTP health
// check against the patroni REST API (GET /primary, 200 only on the leader) so
// writes are routed exclusively to the current primary; without Patroni it falls
// back to a plain TCP connect check against a single fixed master.
func postgresHaproxyConfig(options map[string]string, backends []pgServer, patroni bool) string {
	var b strings.Builder
	// No `daemon`: the systemd unit runs `haproxy -Ws` (master-worker), which is
	// incompatible with the daemon keyword. stats socket exposes show stat/state.
	b.WriteString("global\n  maxconn 4096\n  stats socket /run/haproxy.sock mode 660 level admin\n")
	b.WriteString("defaults\n  mode tcp\n  retries 3\n  timeout connect 5s\n  timeout client 1m\n  timeout server 1m\n")
	// Frontend and backend MUST have distinct proxy names. haproxy treats a
	// frontend and a backend sharing a name as a name clash and the frontend's
	// default_backend then fails to resolve to the real server pool, so the
	// frontend accepts connections but forwards to nothing ("no response" at the
	// client even though the backend servers are UP). Match the master topology's
	// ft_/bk_ naming.
	b.WriteString("frontend ft_postgres\n  bind 0.0.0.0:5432\n  default_backend bk_postgres\n")
	b.WriteString("backend bk_postgres\n")
	if patroni {
		// Patroni leader-aware health check. Patroni REST GET /primary returns 200
		// ONLY on the node holding the leader lock, so haproxy routes writes there.
		// It MUST be sent as HTTP/1.1 WITH a Host header: empirically, patroni's
		// REST returns 200 to an HTTP/1.1+Host request (curl) but NOT to haproxy's
		// legacy HTTP/1.0 no-Host probe (`option httpchk GET /primary`) nor to the
		// bare `option httpchk` OPTIONS probe — both leave every backend marked
		// down ("no response" at the frontend). Use the explicit http-check send.
		b.WriteString("  option httpchk\n")
		b.WriteString("  http-check send meth GET uri /primary ver HTTP/1.1 hdr Host stroppy\n")
		b.WriteString("  http-check expect status 200\n")
		b.WriteString("  default-server inter 3s fall 3 rise 2 on-marked-down shutdown-sessions\n")
		for _, backend := range backends {
			// `check port 8008` keeps the server's host but probes the patroni
			// REST API instead of the postgres/pgbouncer traffic port.
			fmt.Fprintf(&b, "  server %s %s maxconn 100 check port 8008\n", backend.name, backend.addr)
		}
	} else {
		for _, backend := range backends {
			fmt.Fprintf(&b, "  server %s %s check\n", backend.name, backend.addr)
		}
	}
	if extra := renderOptions(options, " "); extra != "" {
		b.WriteString(extra)
	}
	return b.String()
}

// postgresPgbouncerConfig renders pgbouncer.ini pooling the colocated postgres
// instance on the same host (127.0.0.1:5432).
func postgresPgbouncerConfig(componentID string, options map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[databases]\n* = host=127.0.0.1 port=5432\n\n[pgbouncer]\nlisten_addr = 0.0.0.0\nlisten_port = 6432\nauth_type = trust\nauth_file = %s/users.txt\n", deploymentbuilder.ConfigDir(componentID))
	if extra := renderOptions(options, " = "); extra != "" {
		b.WriteString(extra)
	}
	return b.String()
}

// postgresPatroniConfig renders a docs-compliant patroni.yml: Patroni owns the
// postgres instance (initdb on the bootstrap leader, pg_basebackup on followers,
// leader election via etcd) and is the sole supervisor of the data directory.
// The bin_dir is injected at runtime via PATRONI_POSTGRESQL_BIN_DIR (see the
// service unit), so it is intentionally omitted here.
func postgresPatroniConfig(componentID string, database *domain.Database, options map[string]string, wiring postgresWiring) string {
	baseID := strings.TrimSuffix(componentID, "-"+postgresRolePatroni)
	dataDir := postgresDataDir(baseID)
	runDir := postgresRunDir(baseID)
	sync := database.GetParams().GetPostgres().GetSyncReplicas() > 0

	dcs := map[string]string{"ttl": "30", "loop_wait": "10", "retry_timeout": "10"}
	for k, v := range options {
		dcs[k] = v
	}

	var b strings.Builder
	fmt.Fprintf(&b, "scope: stroppy-cluster\nname: %s\n", componentID)
	b.WriteString("restapi:\n  listen: 0.0.0.0:8008\n")
	if wiring.ownHost != "" {
		fmt.Fprintf(&b, "  connect_address: %s:8008\n", wiring.ownHost)
	}
	if len(wiring.etcdClients) > 0 {
		b.WriteString("etcd3:\n  hosts:\n")
		for _, client := range wiring.etcdClients {
			fmt.Fprintf(&b, "  - %s\n", client)
		}
	}

	b.WriteString("bootstrap:\n  dcs:\n")
	fmt.Fprintf(&b, "    ttl: %s\n    loop_wait: %s\n    retry_timeout: %s\n", dcs["ttl"], dcs["loop_wait"], dcs["retry_timeout"])
	b.WriteString("    maximum_lag_on_failover: 1048576\n")
	if sync {
		b.WriteString("    synchronous_mode: true\n")
	}
	b.WriteString("    postgresql:\n      use_pg_rewind: false\n      parameters:\n        listen_addresses: '*'\n        port: 5432\n")
	b.WriteString("  initdb:\n  - encoding: UTF8\n  - data-checksums\n")
	// Trust auth across the ephemeral benchmark cluster, matching the proven
	// master topology: the workload, pgbouncer and replication all connect
	// without passwords, removing scram as a failure variable.
	b.WriteString("  pg_hba:\n")
	b.WriteString("  - local all all trust\n")
	b.WriteString("  - host all all 0.0.0.0/0 trust\n")
	b.WriteString("  - host replication all 0.0.0.0/0 trust\n")

	b.WriteString("postgresql:\n")
	b.WriteString("  listen: 0.0.0.0:5432\n")
	if wiring.ownHost != "" {
		fmt.Fprintf(&b, "  connect_address: %s:5432\n", wiring.ownHost)
	}
	fmt.Fprintf(&b, "  data_dir: %s\n", dataDir)
	fmt.Fprintf(&b, "  pgpass: %s/pgpass\n", runDir)
	b.WriteString("  authentication:\n")
	fmt.Fprintf(&b, "    superuser:\n      username: %s\n      password: %s\n", dbcredentials.PostgresUser, dbcredentials.PostgresPassword)
	fmt.Fprintf(&b, "    replication:\n      username: %s\n      password: %s\n", postgresReplicationUser, postgresReplicationPassword)
	fmt.Fprintf(&b, "  parameters:\n    unix_socket_directories: %s\n", runDir)
	return b.String()
}

func postgresRoleOptions(database *domain.Database, role string) map[string]string {
	params := database.GetParams().GetPostgres()
	switch role {
	case postgresRoleMaster:
		return params.GetMasterOptions()
	case postgresRoleReplica:
		return params.GetReplicaOptions()
	case postgresRoleHaproxy:
		return params.GetHaproxyOptions()
	case postgresRolePgbouncer:
		return params.GetPgbouncerOptions()
	case postgresRolePatroni:
		return params.GetPatroniOptions()
	case postgresRoleEtcd:
		return params.GetEtcdOptions()
	default:
		return nil
	}
}

func configArtifactID(componentID, role string) string {
	return deploymentbuilder.ArtifactID(componentID, configFileName(role))
}
