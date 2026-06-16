package mysql

import (
	"fmt"
	"strings"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

type DeploymentRenderer struct{}

func (r DeploymentRenderer) Supports(component *topologypb.Component) bool {
	return component != nil && component.GetEngine() == mysqlEngine
}

func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	wiring, err := mysqlResolveWiring(ctx)
	if err != nil {
		return nil, err
	}
	ec, err := mysqlEngineComponent(ctx.Component, ctx.Database, ctx.RenderOverrides, ctx.DatabasePackage, deploymentbuilder.DependencyIDs(ctx, nil), wiring)
	if err != nil {
		return nil, err
	}
	return deploymentbuilder.RenderComponentDeployment(ctx, ec), nil
}

func (r DeploymentRenderer) RenderPreview(ctx deploymentbuilder.PreviewContext) ([]*deploymentpb.RenderArtifact, error) {
	deps := deploymentbuilder.DependencyIDs(deploymentbuilder.RenderContext{
		Topology:  ctx.Topology,
		Component: ctx.Component,
		Node:      ctx.Node,
	}, nil)
	// Preview has no runtime addresses; replication/seeds/backends render empty.
	ec, err := mysqlEngineComponent(ctx.Component, ctx.Database, ctx.RenderOverrides, ctx.DatabasePackage, deps, mysqlWiring{})
	if err != nil {
		return nil, err
	}
	return deploymentbuilder.RenderComponentPreview(ctx, ec), nil
}

var mysqlDeploymentSpecs = map[string]struct{ globalPriority, nodePriority uint32 }{
	mysqlRolePrimary:  {globalPriority: 20, nodePriority: 20},
	mysqlRoleReplica:  {globalPriority: 30, nodePriority: 20},
	mysqlRoleProxysql: {globalPriority: 60, nodePriority: 50},
}

// mysqlBackend is one resolved database node behind ProxySQL.
type mysqlBackend struct {
	addr      string
	isPrimary bool
}

// mysqlWiring carries runtime-resolved cluster topology for one component.
type mysqlWiring struct {
	primaryAddr string         // primary mysql host:port (replica replication source)
	ownGroup    string         // own group-replication local address (host:33061)
	groupSeeds  []string       // all members' group-replication host:port (GR)
	backends    []mysqlBackend // db nodes ProxySQL fronts
}

func mysqlResolveWiring(ctx deploymentbuilder.RenderContext) (mysqlWiring, error) {
	wiring := mysqlWiring{}

	switch ctx.Component.GetRole() {
	case mysqlRolePrimary, mysqlRoleReplica:
		seeds, err := deploymentbuilder.ComponentTargets(ctx, isMysqlMember, "group_replication", groupReplicationPort)
		if err != nil {
			return mysqlWiring{}, err
		}
		for _, seed := range seeds {
			wiring.groupSeeds = append(wiring.groupSeeds, seed.AddressPort())
		}
		if own, ok := deploymentbuilder.OwnPrivateEndpoint(ctx); ok {
			wiring.ownGroup = deploymentbuilder.AddressPort(own.Address, groupReplicationPort)
		}
		if ctx.Component.GetRole() == mysqlRoleReplica {
			sources, err := deploymentbuilder.DependencyTargets(ctx, func(_ *topologypb.Connection, target *topologypb.Component) bool {
				return target.GetRole() == mysqlRolePrimary
			})
			if err != nil {
				return mysqlWiring{}, err
			}
			if len(sources) > 0 {
				wiring.primaryAddr = sources[0].AddressPort()
			}
		}
		return wiring, nil
	case mysqlRoleProxysql:
		backends, err := deploymentbuilder.ComponentTargets(ctx, isMysqlMember, "mysql", mysqlPort)
		if err != nil {
			return mysqlWiring{}, err
		}
		for _, backend := range backends {
			wiring.backends = append(wiring.backends, mysqlBackend{
				addr:      backend.AddressPort(),
				isPrimary: backend.Role == mysqlRolePrimary,
			})
		}
		return wiring, nil
	default:
		return wiring, nil
	}
}

func isMysqlMember(c *topologypb.Component) bool {
	return c.GetEngine() == mysqlEngine && (c.GetRole() == mysqlRolePrimary || c.GetRole() == mysqlRoleReplica)
}

func mysqlEngineComponent(
	component *topologypb.Component,
	database *domain.Database,
	overrides *deploymentpb.RenderOverrideSet,
	dbPackage *domain.Package,
	dependencies []string,
	wiring mysqlWiring,
) (deploymentbuilder.EngineComponent, error) {
	spec, ok := mysqlDeploymentSpecs[component.GetRole()]
	if !ok {
		return deploymentbuilder.EngineComponent{}, fmt.Errorf("unsupported mysql component role %q", component.GetRole())
	}

	configFile, configOrigin := mysqlEffectiveConfigFile(component, database, overrides, wiring)
	configDir := deploymentbuilder.ConfigDir(component.GetId())

	ec := deploymentbuilder.EngineComponent{
		Engine:            mysqlEngine,
		GlobalPriority:    spec.globalPriority,
		NodePriority:      spec.nodePriority,
		Dependencies:      dependencies,
		ConfigArtifactID:  configArtifactID(component.GetId(), component.GetRole()),
		ConfigFile:        configFile,
		ConfigOrigin:      configOrigin,
		DefaultConfigFile: mysqlDefaultConfigFile(component, database, mysqlWiring{}),
		InstallCommands:   mysqlInstallCommands(component, dbPackage),
		ServiceFile:       mysqlServiceFile(component.GetId(), component.GetRole(), configDir, database, wiring),
		Healthcheck:       "for i in $(seq 1 60); do if systemctl is-active --quiet " + deploymentbuilder.ShellQuote(deploymentbuilder.ServiceName(component.GetId())) + "; then exit 0; fi; sleep 3; done; exit 1",
	}

	// A replica wires its replication source through a SQL bootstrap applied
	// once the primary is reachable.
	if component.GetRole() == mysqlRoleReplica && wiring.primaryAddr != "" {
		ec.ExtraFiles = []deploymentbuilder.EngineFile{{
			StepID:        "040_write_replication_sql",
			StepOrder:     40,
			ArtifactName:  "replication.sql",
			ArtifactLabel: "replication",
			File: &common.File{
				Info: &common.File_Info{
					Path:          configDir + "/replication.sql",
					Mode:          0644,
					CreateParents: true,
				},
				Content: &common.File_Text{Text: mysqlReplicationSQL(wiring.primaryAddr)},
			},
		}}
	}
	return ec, nil
}

func mysqlInstallCommands(component *topologypb.Component, dbPackage *domain.Package) []string {
	switch component.GetRole() {
	case mysqlRolePrimary, mysqlRoleReplica:
		if dbPackage == nil {
			return []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y mysql-server"}
		}
		commands := append([]string{}, dbPackage.GetPreInstall()...)
		if len(dbPackage.GetAptPackages()) > 0 {
			commands = append(commands, "DEBIAN_FRONTEND=noninteractive apt-get install -y "+strings.Join(dbPackage.GetAptPackages(), " "))
		}
		if dbPackage.GetDebFilename() != "" {
			commands = append(commands, deploymentbuilder.InstallDebFilenameCommand(dbPackage.GetDebFilename(), "/tmp/stroppy-mysql-package.deb"))
		}
		commands = append(commands, "systemctl disable --now mysql || true")
		if len(commands) == 0 {
			commands = append(commands, "true")
		}
		return commands
	case mysqlRoleProxysql:
		// Fetch the .deb through the gateway's /api/binaries cache (pinned upstream
		// in gateway/cache.go), NOT a direct github wget: cloud agents have no
		// direct internet, only the gateway, so the old github download failed with
		// wget exit 4. apt-get install of the local .deb pulls deps via apt-cacher.
		proxysqlDeb := "${STROPPY_SERVER_ADDR%/}/api/binaries/proxysql/2.5.5/proxysql_2.5.5-ubuntu22_amd64.deb"
		return []string{
			"command -v proxysql || (" + deploymentbuilder.CurlDownloadCommand(proxysqlDeb, "/tmp/proxysql.deb") +
				" && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y /tmp/proxysql.deb && rm -f /tmp/proxysql.deb)",
		}
	default:
		return []string{"true"}
	}
}

func mysqlServiceFile(componentID, role, configDir string, database *domain.Database, wiring mysqlWiring) *common.File {
	return deploymentbuilder.EngineServiceFile(componentID, mysqlServiceUnit(componentID, role, configDir, database, wiring))
}

func mysqlServiceUnit(componentID, role, configDir string, database *domain.Database, wiring mysqlWiring) string {
	cfgPath := mysqlConfigPath(componentID, role)
	switch role {
	case mysqlRolePrimary, mysqlRoleReplica:
		dataDir := mysqlDataDir(componentID)
		execStart := "/usr/sbin/mysqld --defaults-file=" + deploymentbuilder.ShellQuote(cfgPath) + " --datadir=" + deploymentbuilder.ShellQuote(dataDir) + " --user=mysql"
		if database.GetParams().GetMysql().GetGroupReplication() {
			if wiring.ownGroup != "" {
				execStart += " --loose-group-replication-local-address=" + deploymentbuilder.ShellQuote(wiring.ownGroup)
			}
			if len(wiring.groupSeeds) > 0 {
				execStart += " --loose-group-replication-group-seeds=" + deploymentbuilder.ShellQuote(strings.Join(wiring.groupSeeds, ","))
			}
		}

		isMariaDB := database.GetKind() == domain.Database_KIND_MARIADB
		initCmd := "/usr/sbin/mysqld --initialize-insecure --datadir=%s --user=mysql"
		if isMariaDB {
			initCmd = "mysql_install_db --datadir=%s --user=mysql"
		}
		execStartPost := ""
		if role == mysqlRolePrimary {
			mysqlClient := "mysql"
			if isMariaDB {
				mysqlClient = "mariadb"
			}
			execStartPost = fmt.Sprintf("ExecStartPost=/bin/sh -ec \"for i in $$(seq 1 30); do if %s -u root -e 'SELECT 1' >/dev/null 2>&1; then %s -u root -e \\\"CREATE USER IF NOT EXISTS 'root'@'%%%%'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%%%%' WITH GRANT OPTION; CREATE DATABASE IF NOT EXISTS stroppy; FLUSH PRIVILEGES;\\\" && exit 0; fi; sleep 1; done; exit 1\"\n", mysqlClient, mysqlClient)
		}
		if role == mysqlRoleReplica && wiring.primaryAddr != "" {
			mysqlClient := "mysql"
			if isMariaDB {
				mysqlClient = "mariadb"
			}
			execStartPost += fmt.Sprintf("ExecStartPost=/bin/sh -c '%s < %s || true'\n", mysqlClient, deploymentbuilder.ShellQuote(configDir+"/replication.sql"))
		}
		socketDir := "ExecStartPre=/bin/sh -ec \"mkdir -p /run/mysqld && chown mysql:mysql /run/mysqld\"\n"
		return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud MySQL %s component %s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=%s/topology.env
ExecStartPre=/bin/mkdir -p %s
ExecStartPre=/bin/chown -R mysql:mysql %s
%sExecStartPre=/bin/sh -ec "if [ ! -d %s/mysql ]; then find %s -mindepth 1 -maxdepth 1 -exec rm -rf {} +; `+initCmd+`; fi"
ExecStart=%s
%sRestart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`,
			role,
			componentID,
			configDir,
			deploymentbuilder.ShellQuote(dataDir),
			deploymentbuilder.ShellQuote(dataDir),
			socketDir,
			deploymentbuilder.ShellQuote(dataDir),
			deploymentbuilder.ShellQuote(dataDir),
			deploymentbuilder.ShellQuote(dataDir),
			execStart,
			execStartPost,
		)
	case mysqlRoleProxysql:
		return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, "/usr/bin/proxysql -f -c "+deploymentbuilder.ShellQuote(cfgPath))
	default:
		return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, "/bin/false")
	}
}

func mysqlReplicationSQL(primaryAddr string) string {
	host := primaryAddr
	port := mysqlPort
	if h, p, ok := splitHostPort(primaryAddr); ok {
		host = h
		port = p
	}
	return fmt.Sprintf("CHANGE REPLICATION SOURCE TO SOURCE_HOST='%s', SOURCE_PORT=%d, SOURCE_USER='root', SOURCE_AUTO_POSITION=1;\nSTART REPLICA;\n", host, port)
}

func splitHostPort(addr string) (string, int, bool) {
	idx := strings.LastIndex(addr, ":")
	if idx < 0 || strings.HasPrefix(addr, "[") {
		return "", 0, false
	}
	host := addr[:idx]
	var port int
	if _, err := fmt.Sscanf(addr[idx+1:], "%d", &port); err != nil {
		return "", 0, false
	}
	return host, port, true
}

func mysqlEffectiveConfigFile(component *topologypb.Component, database *domain.Database, overrides *deploymentpb.RenderOverrideSet, wiring mysqlWiring) (*common.File, deploymentpb.RenderArtifact_Origin) {
	artifactID := configArtifactID(component.GetId(), component.GetRole())
	if override, ok := deploymentbuilder.OverrideFile(overrides, component.GetId(), artifactID); ok && override.GetFile() != nil {
		return override.GetFile(), deploymentpb.RenderArtifact_ORIGIN_USER_OVERRIDE
	}
	return mysqlDefaultConfigFile(component, database, wiring), deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT
}

func mysqlDefaultConfigFile(component *topologypb.Component, database *domain.Database, wiring mysqlWiring) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          mysqlConfigPath(component.GetId(), component.GetRole()),
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: mysqlConfigContentForRole(database, component.GetId(), component.GetRole(), wiring)},
	}
}

func mysqlConfigContentForRole(database *domain.Database, componentID, role string, wiring mysqlWiring) string {
	params := database.GetParams().GetMysql()
	isMariaDB := database.GetKind() == domain.Database_KIND_MARIADB
	switch role {
	case mysqlRolePrimary:
		return mysqlConfigContent(params, componentID, role, params.GetPrimaryOptions(), isMariaDB)
	case mysqlRoleReplica:
		return mysqlConfigContent(params, componentID, role, params.GetReplicaOptions(), isMariaDB)
	case mysqlRoleProxysql:
		return mysqlProxysqlConfig(params.GetProxysqlOptions(), wiring.backends)
	default:
		return ""
	}
}

// mysqlProxysqlConfig renders a proxysql.cnf with the resolved mysql_servers
// (hostgroup 0 = primary writer, 1 = replica readers).
func mysqlProxysqlConfig(options map[string]string, backends []mysqlBackend) string {
	var b strings.Builder
	b.WriteString("datadir=\"/var/lib/proxysql\"\n")
	fmt.Fprintf(&b, "mysql_variables=\n{\n  interfaces=\"0.0.0.0:%d\"\n}\n", proxysqlPort)
	b.WriteString("mysql_servers=\n(\n")
	for _, backend := range backends {
		hostgroup := 1
		if backend.isPrimary {
			hostgroup = 0
		}
		host, port := backend.addr, mysqlPort
		if h, p, ok := splitHostPort(backend.addr); ok {
			host = h
			port = p
		}
		fmt.Fprintf(&b, "  { address=\"%s\" , port=%d , hostgroup=%d },\n", host, port, hostgroup)
	}
	b.WriteString(")\n")
	b.WriteString("mysql_users=\n(\n")
	b.WriteString("  { username=\"root\" , default_hostgroup=0 , active=1 },\n")
	b.WriteString(")\n")
	if len(options) > 0 {
		keys := sortedKeys(options)
		for _, key := range keys {
			fmt.Fprintf(&b, "%s=%s\n", key, options[key])
		}
	}
	return b.String()
}

func mysqlConfigPath(componentID, role string) string {
	if role == mysqlRoleProxysql {
		return deploymentbuilder.ConfigDir(componentID) + "/" + mysqlConfigFileName(role)
	}
	return "/etc/mysql/stroppy-cloud/" + componentID + ".cnf"
}

func mysqlConfigFileName(role string) string {
	if role == mysqlRoleProxysql {
		return "proxysql.cnf"
	}
	return "my.cnf"
}

func configArtifactID(componentID, role string) string {
	return deploymentbuilder.ArtifactID(componentID, mysqlConfigFileName(role))
}
