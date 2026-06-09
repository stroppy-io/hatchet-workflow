package ydb

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbspec"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

// defaultYdbDownloadURL is the stock ydbd release tarball used when no package
// override supplies one. YDB ships a binary, not an apt package.
const defaultYdbDownloadURL = "${STROPPY_SERVER_ADDR%/}/api/binaries/ydbd/24.1.18/ydbd-24.1.18-linux-amd64.tar.gz"

type DeploymentRenderer struct{}

func (r DeploymentRenderer) Supports(component *topologypb.Component) bool {
	return component != nil && component.GetEngine() == ydbEngine
}

func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	wiring, err := ydbResolveWiring(ctx)
	if err != nil {
		return nil, err
	}
	ec, err := ydbEngineComponent(ctx.Component, ctx.Database, ctx.RenderOverrides, ctx.DatabasePackage, deploymentbuilder.DependencyIDs(ctx, nil), wiring)
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
	// Preview has no runtime addresses; hosts/node-broker/backends render empty.
	ec, err := ydbEngineComponent(ctx.Component, ctx.Database, ctx.RenderOverrides, ctx.DatabasePackage, deps, ydbWiring{})
	if err != nil {
		return nil, err
	}
	return deploymentbuilder.RenderComponentPreview(ctx, ec), nil
}

var ydbDeploymentSpecs = map[string]struct{ globalPriority, nodePriority uint32 }{
	ydbRoleStorage:  {globalPriority: 10, nodePriority: 10},
	ydbRoleDatabase: {globalPriority: 20, nodePriority: 20},
	ydbRoleHaproxy:  {globalPriority: 60, nodePriority: 50},
}

// ydbWiring carries runtime-resolved cluster topology for one component.
type ydbWiring struct {
	storageHosts []string // storage node addresses (config.yaml hosts:)
	nodeBrokers  []string // storage grpc host:port (dynamic --node-broker)
	backends     []string // grpc host:port the haproxy fronts
}

func ydbResolveWiring(ctx deploymentbuilder.RenderContext) (ydbWiring, error) {
	storageIC, err := deploymentbuilder.ComponentTargets(ctx, isYdbStorage, "ic", icPort)
	if err != nil {
		return ydbWiring{}, err
	}
	storageBrokers, err := deploymentbuilder.ComponentTargets(ctx, isYdbStorage, "node-broker", storageGrpcPort)
	if err != nil {
		return ydbWiring{}, err
	}
	databaseGRPC, err := deploymentbuilder.ComponentTargets(ctx, isYdbDatabase, "grpc", grpcPort)
	if err != nil {
		return ydbWiring{}, err
	}
	storageGRPC, err := deploymentbuilder.ComponentTargets(ctx, isYdbStorage, "grpc", grpcPort)
	if err != nil {
		return ydbWiring{}, err
	}

	wiring := ydbWiring{}
	for _, target := range storageIC {
		wiring.storageHosts = append(wiring.storageHosts, target.Address)
	}
	for _, target := range storageBrokers {
		wiring.nodeBrokers = append(wiring.nodeBrokers, target.AddressPort())
	}

	// HAProxy fronts the compute tier; in combined mode that is the storage nodes.
	frontends := databaseGRPC
	if len(frontends) == 0 {
		frontends = storageGRPC
	}
	for _, target := range frontends {
		wiring.backends = append(wiring.backends, target.AddressPort())
	}
	return wiring, nil
}

func isYdbStorage(c *topologypb.Component) bool {
	return c.GetEngine() == ydbEngine && c.GetRole() == ydbRoleStorage
}

func isYdbDatabase(c *topologypb.Component) bool {
	return c.GetEngine() == ydbEngine && c.GetRole() == ydbRoleDatabase
}

func ydbEngineComponent(
	component *topologypb.Component,
	database *domain.Database,
	overrides *deploymentpb.RenderOverrideSet,
	dbPackage *domain.Package,
	dependencies []string,
	wiring ydbWiring,
) (deploymentbuilder.EngineComponent, error) {
	spec, ok := ydbDeploymentSpecs[component.GetRole()]
	if !ok {
		return deploymentbuilder.EngineComponent{}, fmt.Errorf("unsupported ydb component role %q", component.GetRole())
	}

	configFile, configOrigin := ydbEffectiveConfigFile(component, database, overrides, wiring)
	configDir := deploymentbuilder.ConfigDir(component.GetId())

	ec := deploymentbuilder.EngineComponent{
		Engine:            ydbEngine,
		GlobalPriority:    spec.globalPriority,
		NodePriority:      spec.nodePriority,
		Dependencies:      dependencies,
		ConfigArtifactID:  configArtifactID(component.GetId(), component.GetRole()),
		ConfigFile:        configFile,
		ConfigOrigin:      configOrigin,
		DefaultConfigFile: ydbDefaultConfigFile(component, database, ydbWiring{}),
		InstallCommands:   ydbInstallCommands(component, dbPackage),
		ServiceFile:       ydbServiceFile(component.GetId(), component.GetRole(), configDir, database, wiring),
		Healthcheck:       ydbHealthcheckCommand(component, database),
		PostStartCommands: ydbPostStartCommands(component, database),
	}
	if ydbIsCombined(database) && component.GetRole() == ydbRoleStorage {
		ec.ExtraFiles = append(ec.ExtraFiles,
			deploymentbuilder.EngineFile{
				StepID:        "040_write_database_config",
				StepOrder:     40,
				ArtifactName:  "database.yaml",
				ArtifactLabel: "database_config",
				File:          ydbCombinedDatabaseConfigFile(component.GetId(), database, wiring),
			},
			deploymentbuilder.EngineFile{
				StepID:        "050_write_database_service",
				StepOrder:     50,
				ArtifactName:  "database.service",
				ArtifactLabel: "database_service",
				LockReason:    "combined YDB database service is generated by the deployment renderer",
				File:          ydbCombinedDatabaseServiceFile(component.GetId(), configDir, database, wiring),
			},
		)
	}
	return ec, nil
}

func ydbInstallCommands(component *topologypb.Component, dbPackage *domain.Package) []string {
	switch component.GetRole() {
	case ydbRoleStorage, ydbRoleDatabase:
		url := defaultYdbDownloadURL
		var commands []string
		if dbPackage != nil {
			commands = append(commands, dbPackage.GetPreInstall()...)
			if dbPackage.GetDebFilename() != "" {
				url = dbPackage.GetDebFilename()
			}
		}
		commands = append(commands,
			deploymentbuilder.CurlDownloadCommand(url, "/tmp/ydbd.tgz")+" && tar -xzf /tmp/ydbd.tgz -C /opt && install -m 0755 \"$(find /opt -path '*/bin/ydbd' | head -n1)\" /usr/local/bin/ydbd",
		)
		return commands
	case ydbRoleHaproxy:
		return []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y haproxy"}
	default:
		return []string{"true"}
	}
}

func ydbPostStartCommands(component *topologypb.Component, database *domain.Database) []deploymentbuilder.EngineCommand {
	if component.GetRole() != ydbRoleStorage {
		return nil
	}
	var commands []deploymentbuilder.EngineCommand
	if component.GetId() == "ydb-storage-1" {
		commands = append(commands, deploymentbuilder.EngineCommand{
			StepID:        "240_init_database",
			StepOrder:     240,
			ArtifactName:  "bootstrap/init-database",
			ArtifactLabel: "bootstrap",
			LockReason:    "YDB cluster and tenant database bootstrap is generated by the deployment renderer",
			Command:       ydbInitDatabaseCommand(database),
		})
	}
	if ydbIsCombined(database) {
		serviceName := ydbCombinedDatabaseServiceName(component.GetId())
		commands = append(commands,
			deploymentbuilder.EngineCommand{
				StepID:        "250_enable_start_database",
				StepOrder:     250,
				ArtifactName:  "systemd/enable-start-database",
				ArtifactLabel: "systemd_enable_start",
				LockReason:    "combined YDB database service activation is renderer-owned",
				Command:       ydbEnableStartServiceCommand(serviceName),
			},
			deploymentbuilder.EngineCommand{
				StepID:        "260_database_healthcheck",
				StepOrder:     260,
				ArtifactName:  "healthcheck/database",
				ArtifactLabel: "healthcheck",
				LockReason:    "combined YDB database service healthcheck is renderer-owned",
				Command:       ydbDatabaseHealthcheckCommand(serviceName, database),
			},
		)
	}
	return commands
}

func ydbInitDatabaseCommand(database *domain.Database) string {
	params := database.GetParams().GetYdb()
	databasePath := ydbDatabasePath(params)
	storageGroups := params.GetStorageGroups()
	if storageGroups == 0 {
		storageGroups = 1
	}
	pool := fmt.Sprintf("%s:%d", ydbDiskType(params), storageGroups)
	configPath := ydbConfigPath("ydb-storage-1", ydbRoleStorage)
	server := fmt.Sprintf("grpc://127.0.0.1:%d", storageGrpcPort)

	return fmt.Sprintf(`set -e
server=%s
database=%s
pool=%s
config=%s

last_log=/tmp/stroppy-ydb-bootstrap.log
for attempt in $(seq 1 60); do
  if /usr/local/bin/ydbd -s "$server" admin database "$database" status >/dev/null 2>&1; then
    echo "YDB database $database already exists"
    exit 0
  fi

  if /usr/local/bin/ydbd -s "$server" admin blobstorage config init --yaml-file "$config" >>"$last_log" 2>&1; then
    echo "YDB blobstorage config initialized"
  else
    cat "$last_log" >&2 || true
  fi

  if /usr/local/bin/ydbd -s "$server" admin database "$database" create "$pool" >>"$last_log" 2>&1; then
    echo "YDB database $database created with pool $pool"
    exit 0
  fi
  cat "$last_log" >&2 || true
  sleep 5
done

echo "YDB database $database was not created after retries" >&2
exit 1
`, deploymentbuilder.ShellQuote(server), deploymentbuilder.ShellQuote(databasePath), deploymentbuilder.ShellQuote(pool), deploymentbuilder.ShellQuote(configPath))
}

func ydbServiceFile(componentID, role, configDir string, database *domain.Database, wiring ydbWiring) *common.File {
	return deploymentbuilder.EngineServiceFile(componentID, ydbServiceUnit(componentID, role, configDir, database, wiring))
}

func ydbHealthcheckCommand(component *topologypb.Component, database *domain.Database) string {
	serviceName := deploymentbuilder.ServiceName(component.GetId())
	if component.GetRole() == ydbRoleDatabase {
		return ydbDatabaseHealthcheckCommand(serviceName, database)
	}
	return "systemctl is-active --quiet " + deploymentbuilder.ShellQuote(serviceName)
}

func ydbDatabaseHealthcheckCommand(serviceName string, database *domain.Database) string {
	endpoint := fmt.Sprintf("grpc://127.0.0.1:%d", grpcPort)
	databasePath := ydbDatabasePath(database.GetParams().GetYdb())
	return fmt.Sprintf(`set -e
service=%s
endpoint=%s
database=%s

for attempt in $(seq 1 180); do
  if systemctl is-active --quiet "$service" && /usr/local/bin/ydbd -s "$endpoint" admin database "$database" status >/dev/null 2>&1; then
    # The admin endpoint can become available a little before the query service
    # is ready for SDK session creation. Give the dynamic node a short settle
    # window so stroppy does not fall through its 3s primary connect timeout.
    sleep 20
    exit 0
  fi
  sleep 2
done

echo "--- systemctl status $service ---"
systemctl status --no-pager -l "$service" || true
echo "--- journalctl $service ---"
journalctl --no-pager --output=short-iso-precise -u "$service" -n 200 || true
exit 1
`, deploymentbuilder.ShellQuote(serviceName), deploymentbuilder.ShellQuote(endpoint), deploymentbuilder.ShellQuote(databasePath))
}

func ydbServiceUnit(componentID, role, configDir string, database *domain.Database, wiring ydbWiring) string {
	cfgPath := ydbConfigPath(componentID, role)
	dataDir := deploymentbuilder.DataDir(componentID)

	switch role {
	case ydbRoleStorage:
		execStart := fmt.Sprintf("/usr/local/bin/ydbd server --yaml-config %s --grpc-port %d --ic-port %d --mon-port %d --node %d",
			deploymentbuilder.ShellQuote(cfgPath), storageGrpcPort, icPort, monPort, ydbStaticNodeID(componentID))
		return ydbDaemonServiceUnit(componentID, role, configDir, dataDir, execStart)
	case ydbRoleDatabase:
		tenant := ydbDatabasePath(database.GetParams().GetYdb())
		execStart := fmt.Sprintf("/usr/local/bin/ydbd server --yaml-config %s --grpc-port %d --ic-port %d --mon-port %d --tenant %s",
			deploymentbuilder.ShellQuote(cfgPath), grpcPort, databaseICPort, databaseMonPort, deploymentbuilder.ShellQuote(tenant))
		for _, broker := range wiring.nodeBrokers {
			execStart += " --node-broker " + deploymentbuilder.ShellQuote(broker)
		}
		return ydbDaemonServiceUnit(componentID, role, configDir, dataDir, execStart)
	case ydbRoleHaproxy:
		return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, "/usr/sbin/haproxy -Ws -f "+deploymentbuilder.ShellQuote(cfgPath))
	default:
		return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, "/bin/false")
	}
}

func ydbIsCombined(database *domain.Database) bool {
	return database.GetParams().GetYdb().GetDatabaseNodes() == 0
}

func ydbCombinedDatabaseConfigFile(componentID string, database *domain.Database, wiring ydbWiring) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          ydbCombinedDatabaseConfigPath(componentID),
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{
			Text: ydbConfigContentForRole(database, ydbRoleDatabase, wiring),
		},
	}
}

func ydbCombinedDatabaseServiceFile(componentID, configDir string, database *domain.Database, wiring ydbWiring) *common.File {
	serviceName := ydbCombinedDatabaseServiceName(componentID)
	tenant := ydbDatabasePath(database.GetParams().GetYdb())
	execStart := fmt.Sprintf("/usr/local/bin/ydbd server --yaml-config %s --grpc-port %d --ic-port %d --mon-port %d --tenant %s",
		deploymentbuilder.ShellQuote(ydbCombinedDatabaseConfigPath(componentID)), grpcPort, databaseICPort, databaseMonPort, deploymentbuilder.ShellQuote(tenant))
	for _, broker := range wiring.nodeBrokers {
		execStart += " --node-broker " + deploymentbuilder.ShellQuote(broker)
	}
	return &common.File{
		Info: &common.File_Info{
			Path:          "/etc/systemd/system/" + serviceName + ".service",
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{
			Text: ydbDaemonNamedServiceUnit(serviceName, componentID, ydbRoleDatabase, configDir, deploymentbuilder.DataDir(componentID)+"-database", execStart),
		},
	}
}

func ydbCombinedDatabaseConfigPath(componentID string) string {
	return deploymentbuilder.ConfigDir(componentID) + "/database.yaml"
}

func ydbCombinedDatabaseServiceName(componentID string) string {
	return deploymentbuilder.ServiceName(componentID) + "-database"
}

func ydbStaticNodeID(componentID string) int {
	const prefix = "ydb-storage-"
	if strings.HasPrefix(componentID, prefix) {
		if id, err := strconv.Atoi(strings.TrimPrefix(componentID, prefix)); err == nil && id > 0 {
			return id
		}
	}
	return 1
}

func ydbDaemonServiceUnit(componentID, role, configDir, dataDir, execStart string) string {
	return ydbDaemonNamedServiceUnit(deploymentbuilder.ServiceName(componentID), componentID, role, configDir, dataDir, execStart)
}

func ydbDaemonNamedServiceUnit(serviceName, componentID, role, configDir, dataDir, execStart string) string {
	after := "network-online.target"
	wants := "network-online.target"
	if role == ydbRoleDatabase && strings.HasSuffix(serviceName, "-database") {
		storageService := deploymentbuilder.ServiceName(componentID) + ".service"
		after += " " + storageService
		wants += " " + storageService
	}
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud YDB %s component %s
After=%s
Wants=%s

[Service]
Type=simple
EnvironmentFile=%s/topology.env
ExecStartPre=/bin/mkdir -p %s
ExecStartPre=/bin/mkdir -p %s
ExecStart=%s
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`, role, componentID, after, wants, configDir, deploymentbuilder.ShellQuote(dataDir), deploymentbuilder.ShellQuote(ydbPDiskDir), execStart)
}

func ydbEnableStartServiceCommand(serviceName string) string {
	service := deploymentbuilder.ShellQuote(serviceName)
	return fmt.Sprintf(`set -e
if ! systemctl enable --now %s; then
  echo "--- systemctl status %s ---"
  systemctl status --no-pager -l %s || true
  echo "--- journalctl %s ---"
  journalctl --no-pager --output=short-iso-precise -u %s -n 200 || true
  exit 1
fi
`, service, service, service, service, service)
}

func ydbEffectiveConfigFile(component *topologypb.Component, database *domain.Database, overrides *deploymentpb.RenderOverrideSet, wiring ydbWiring) (*common.File, deploymentpb.RenderArtifact_Origin) {
	artifactID := configArtifactID(component.GetId(), component.GetRole())
	if override, ok := deploymentbuilder.OverrideFile(overrides, component.GetId(), artifactID); ok && override.GetFile() != nil {
		return override.GetFile(), deploymentpb.RenderArtifact_ORIGIN_USER_OVERRIDE
	}
	return ydbDefaultConfigFile(component, database, wiring), deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT
}

func ydbDefaultConfigFile(component *topologypb.Component, database *domain.Database, wiring ydbWiring) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          ydbConfigPath(component.GetId(), component.GetRole()),
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: ydbConfigContentForRole(database, component.GetRole(), wiring)},
	}
}

func ydbConfigContentForRole(database *domain.Database, role string, wiring ydbWiring) string {
	params := database.GetParams().GetYdb()
	switch role {
	case ydbRoleStorage:
		return ydbConfigContent(params, "STORAGE", params.GetStorageOptions(), wiring.storageHosts)
	case ydbRoleDatabase:
		return ydbConfigContent(params, "COMPUTE", params.GetDatabaseOptions(), wiring.storageHosts)
	case ydbRoleHaproxy:
		return ydbHaproxyConfig(params.GetHaproxyOptions(), wiring.backends)
	default:
		return ""
	}
}

// ydbHaproxyConfig renders an haproxy.cfg balancing the gRPC port across the
// resolved compute (or storage) backends.
func ydbHaproxyConfig(options map[string]string, backends []string) string {
	var b strings.Builder
	b.WriteString("global\n  daemon\n")
	b.WriteString("defaults\n  mode tcp\n  timeout connect 5s\n  timeout client 1m\n  timeout server 1m\n")
	fmt.Fprintf(&b, "frontend grpc\n  bind 0.0.0.0:%d\n  default_backend ydb\n", grpcPort)
	b.WriteString("backend ydb\n")
	for i, backend := range backends {
		fmt.Fprintf(&b, "  server ydb-%d %s check\n", i+1, backend)
	}
	if extra := dbspec.RenderOptions(options, " "); extra != "" {
		b.WriteString(extra)
	}
	return b.String()
}

func ydbConfigPath(componentID, role string) string {
	return deploymentbuilder.ConfigDir(componentID) + "/" + ydbConfigFileName(role)
}

func ydbConfigFileName(role string) string {
	if role == ydbRoleHaproxy {
		return "haproxy.cfg"
	}
	return "config.yaml"
}

func configArtifactID(componentID, role string) string {
	return deploymentbuilder.ArtifactID(componentID, ydbConfigFileName(role))
}
