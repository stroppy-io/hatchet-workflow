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
	wiring := postgresWiring{}

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
		Healthcheck:       postgresHealthcheckCommand(component),
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
				commands = append(commands, "DEBIAN_FRONTEND=noninteractive apt-get install -y "+deploymentbuilder.ShellQuote(dbPackage.GetDebFilename()))
			}
		}
		commands = append(commands, "systemctl disable --now postgresql || true")
		if len(commands) == 0 {
			commands = append(commands, "true")
		}
		return commands
	case postgresRoleHaproxy:
		return []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y haproxy"}
	case postgresRolePgbouncer:
		return []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y pgbouncer"}
	case postgresRolePatroni:
		return []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y patroni"}
	case postgresRoleEtcd:
		return []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y etcd"}
	default:
		return []string{"true"}
	}
}

func postgresServiceFile(componentID, role, configDir string, wiring postgresWiring) *common.File {
	return deploymentbuilder.EngineServiceFile(componentID, postgresServiceUnit(componentID, role, configDir, wiring))
}

func postgresServiceUnit(componentID, role, configDir string, wiring postgresWiring) string {
	cfgPath := configPath(componentID, role)
	switch role {
	case postgresRoleMaster:
		return postgresMasterServiceUnit(componentID, role, configDir, cfgPath)
	case postgresRoleReplica:
		if wiring.masterAddr != "" {
			return postgresReplicaServiceUnit(componentID, role, configDir, cfgPath, wiring.masterAddr)
		}
		return postgresDatabaseServiceUnit(componentID, role, configDir, cfgPath)
	case postgresRoleHaproxy:
		return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, "/usr/sbin/haproxy -Ws -f "+deploymentbuilder.ShellQuote(cfgPath))
	case postgresRolePgbouncer:
		return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, "/usr/sbin/pgbouncer "+deploymentbuilder.ShellQuote(cfgPath))
	case postgresRolePatroni:
		return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, "/usr/bin/patroni "+deploymentbuilder.ShellQuote(cfgPath))
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
	return base + fmt.Sprintf(`ExecStartPost=/bin/sh -ec "for i in $$(seq 1 30); do if pg_isready -h 127.0.0.1 -p 5432; then exec /usr/sbin/runuser -u postgres -- psql -v ON_ERROR_STOP=1 -h 127.0.0.1 -p 5432 -f %s; fi; sleep 1; done; exit 1"

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
ExecStartPre=/bin/mkdir -p %s %s
ExecStartPre=/bin/chown -R postgres:postgres %s %s
ExecStartPre=/bin/chown postgres:postgres %s
ExecStartPre=/bin/chmod 0600 %s
ExecStartPre=/bin/sh -ec "test -s %s/PG_VERSION || /usr/sbin/runuser -u postgres -- env PGPASSFILE=%s \"$$(find /usr/lib/postgresql -path '*/bin/pg_basebackup' | sort -V | tail -n1)\" -h %s -p %d -U %s -w -D %s -R"
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
END
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
		return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, "/usr/bin/etcd --config-file "+deploymentbuilder.ShellQuote(cfgPath))
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
	return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, execStart)
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

func postgresHealthcheckCommand(component *topologypb.Component) string {
	service := deploymentbuilder.ShellQuote(deploymentbuilder.ServiceName(component.GetId()))
	switch component.GetRole() {
	case postgresRoleMaster, postgresRoleReplica:
		return "systemctl is-active --quiet " + service + " && pg_isready -h 127.0.0.1 -p 5432"
	case postgresRolePgbouncer:
		return "systemctl is-active --quiet " + service + " && pg_isready -h 127.0.0.1 -p 6432"
	default:
		return "systemctl is-active --quiet " + service
	}
}

func postgresDataDir(componentID string) string {
	return "/var/lib/stroppy-cloud/" + componentID
}

func postgresRunDir(componentID string) string {
	return "/run/stroppy-cloud/" + componentID
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
		Content: &common.File_Text{Text: postgresConfigContentForRole(database, component.GetRole(), wiring)},
	}
}

func postgresConfigContentForRole(database *domain.Database, role string, wiring postgresWiring) string {
	options := postgresRoleOptions(database, role)
	switch role {
	case postgresRoleHaproxy:
		return postgresHaproxyConfig(options, wiring.backends)
	case postgresRolePgbouncer:
		return postgresPgbouncerConfig(options)
	case postgresRolePatroni:
		return postgresPatroniConfig(options, wiring.etcdClients)
	default:
		return configContent(role, options)
	}
}

// postgresHaproxyConfig renders an haproxy.cfg fronting port 5432 across the
// resolved pg/pgbouncer backends.
func postgresHaproxyConfig(options map[string]string, backends []pgServer) string {
	var b strings.Builder
	b.WriteString("global\n  daemon\n")
	b.WriteString("defaults\n  mode tcp\n  timeout connect 5s\n  timeout client 1m\n  timeout server 1m\n")
	b.WriteString("frontend postgres\n  bind 0.0.0.0:5432\n  default_backend postgres\n")
	b.WriteString("backend postgres\n")
	for _, backend := range backends {
		fmt.Fprintf(&b, "  server %s %s check\n", backend.name, backend.addr)
	}
	if extra := renderOptions(options, " "); extra != "" {
		b.WriteString(extra)
	}
	return b.String()
}

// postgresPgbouncerConfig renders pgbouncer.ini pooling the colocated postgres
// instance on the same host (127.0.0.1:5432).
func postgresPgbouncerConfig(options map[string]string) string {
	var b strings.Builder
	b.WriteString("[databases]\n* = host=127.0.0.1 port=5432\n\n[pgbouncer]\nlisten_addr = 0.0.0.0\nlisten_port = 6432\n")
	if extra := renderOptions(options, " = "); extra != "" {
		b.WriteString(extra)
	}
	return b.String()
}

// postgresPatroniConfig renders a patroni.yml with the resolved etcd hosts.
func postgresPatroniConfig(options map[string]string, etcdClients []string) string {
	var b strings.Builder
	b.WriteString("scope: stroppy-cluster\n")
	if len(etcdClients) > 0 {
		b.WriteString("etcd3:\n  hosts:\n")
		for _, client := range etcdClients {
			fmt.Fprintf(&b, "  - %s\n", client)
		}
	}
	if extra := renderOptions(options, ": "); extra != "" {
		b.WriteString(extra)
	}
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
