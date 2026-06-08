package ydb

import (
	"fmt"
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
	storageGRPC, err := deploymentbuilder.ComponentTargets(ctx, isYdbStorage, "grpc", grpcPort)
	if err != nil {
		return ydbWiring{}, err
	}
	databaseGRPC, err := deploymentbuilder.ComponentTargets(ctx, isYdbDatabase, "grpc", grpcPort)
	if err != nil {
		return ydbWiring{}, err
	}

	wiring := ydbWiring{}
	for _, target := range storageIC {
		wiring.storageHosts = append(wiring.storageHosts, target.Address)
	}
	for _, target := range storageGRPC {
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

	return deploymentbuilder.EngineComponent{
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
		Healthcheck:       "systemctl is-active --quiet " + deploymentbuilder.ShellQuote(deploymentbuilder.ServiceName(component.GetId())),
	}, nil
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
			"curl -fsSL "+deploymentbuilder.ShellQuote(url)+" -o /tmp/ydbd.tgz && tar -xzf /tmp/ydbd.tgz -C /opt && install -m 0755 \"$(find /opt -path '*/bin/ydbd' | head -n1)\" /usr/local/bin/ydbd",
		)
		return commands
	case ydbRoleHaproxy:
		return []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y haproxy"}
	default:
		return []string{"true"}
	}
}

func ydbServiceFile(componentID, role, configDir string, database *domain.Database, wiring ydbWiring) *common.File {
	return deploymentbuilder.EngineServiceFile(componentID, ydbServiceUnit(componentID, role, configDir, database, wiring))
}

func ydbServiceUnit(componentID, role, configDir string, database *domain.Database, wiring ydbWiring) string {
	cfgPath := ydbConfigPath(componentID, role)
	dataDir := deploymentbuilder.DataDir(componentID)

	switch role {
	case ydbRoleStorage:
		execStart := fmt.Sprintf("/usr/local/bin/ydbd server --yaml-config %s --grpc-port %d --ic-port %d --mon-port %d --node static",
			deploymentbuilder.ShellQuote(cfgPath), grpcPort, icPort, monPort)
		return ydbDaemonServiceUnit(componentID, role, configDir, dataDir, execStart)
	case ydbRoleDatabase:
		tenant := ydbDatabasePath(database.GetParams().GetYdb())
		execStart := fmt.Sprintf("/usr/local/bin/ydbd server --yaml-config %s --grpc-port %d --ic-port %d --mon-port %d --tenant %s --node dynamic",
			deploymentbuilder.ShellQuote(cfgPath), grpcPort, icPort, monPort, deploymentbuilder.ShellQuote(tenant))
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

func ydbDaemonServiceUnit(componentID, role, configDir, dataDir, execStart string) string {
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud YDB %s component %s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=%s/topology.env
ExecStartPre=/bin/mkdir -p %s
ExecStart=%s
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`, role, componentID, configDir, deploymentbuilder.ShellQuote(dataDir), execStart)
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
