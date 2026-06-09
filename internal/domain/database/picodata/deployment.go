package picodata

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

type DeploymentRenderer struct{}

func (r DeploymentRenderer) Supports(component *topologypb.Component) bool {
	return component != nil && component.GetEngine() == picodataEngine
}

func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	wiring, err := picodataResolveWiring(ctx)
	if err != nil {
		return nil, err
	}
	ec, err := picodataEngineComponent(ctx.Component, ctx.Database, ctx.RenderOverrides, ctx.DatabasePackage, deploymentbuilder.DependencyIDs(ctx, nil), wiring)
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
	// Preview has no runtime addresses; the unit/backends render without peers.
	ec, err := picodataEngineComponent(ctx.Component, ctx.Database, ctx.RenderOverrides, ctx.DatabasePackage, deps, picodataWiring{})
	if err != nil {
		return nil, err
	}
	return deploymentbuilder.RenderComponentPreview(ctx, ec), nil
}

var picodataDeploymentSpecs = map[string]struct{ globalPriority, nodePriority uint32 }{
	picodataRoleInstance: {globalPriority: 20, nodePriority: 20},
	picodataRoleHaproxy:  {globalPriority: 60, nodePriority: 50},
}

// picodataWiring carries runtime-resolved cluster topology for one component.
type picodataWiring struct {
	advertise   string   // own iproto host:port (instances)
	peer        string   // bootstrap instance iproto host:port (empty on the bootstrap)
	isBootstrap bool     // first instance bootstraps the raft group
	backends    []string // instance pgproto host:port list (haproxy)
}

func picodataResolveWiring(ctx deploymentbuilder.RenderContext) (picodataWiring, error) {
	instances, err := deploymentbuilder.ComponentTargets(ctx, func(c *topologypb.Component) bool {
		return c.GetEngine() == picodataEngine && c.GetRole() == picodataRoleInstance
	}, "iproto", iprotoPort)
	if err != nil {
		return picodataWiring{}, err
	}

	switch ctx.Component.GetRole() {
	case picodataRoleInstance:
		own, ok := deploymentbuilder.OwnPrivateEndpoint(ctx)
		if !ok {
			return picodataWiring{}, fmt.Errorf("picodata instance %q has no private endpoint", ctx.Component.GetId())
		}
		wiring := picodataWiring{advertise: deploymentbuilder.AddressPort(own.Address, iprotoPort)}
		if len(instances) > 0 && instances[0].ComponentID == ctx.Component.GetId() {
			wiring.isBootstrap = true
		} else if len(instances) > 0 {
			wiring.peer = instances[0].AddressPort()
		}
		return wiring, nil
	case picodataRoleHaproxy:
		backends, err := deploymentbuilder.ComponentTargets(ctx, func(c *topologypb.Component) bool {
			return c.GetEngine() == picodataEngine && c.GetRole() == picodataRoleInstance
		}, "pgproto", pgprotoPort)
		if err != nil {
			return picodataWiring{}, err
		}
		addrs := make([]string, 0, len(backends))
		for _, backend := range backends {
			addrs = append(addrs, backend.AddressPort())
		}
		return picodataWiring{backends: addrs}, nil
	default:
		return picodataWiring{}, nil
	}
}

func picodataEngineComponent(
	component *topologypb.Component,
	database *domain.Database,
	overrides *deploymentpb.RenderOverrideSet,
	dbPackage *domain.Package,
	dependencies []string,
	wiring picodataWiring,
) (deploymentbuilder.EngineComponent, error) {
	spec, ok := picodataDeploymentSpecs[component.GetRole()]
	if !ok {
		return deploymentbuilder.EngineComponent{}, fmt.Errorf("unsupported picodata component role %q", component.GetRole())
	}

	configFile, configOrigin := picodataEffectiveConfigFile(component, database, overrides, wiring)
	configDir := deploymentbuilder.ConfigDir(component.GetId())

	return deploymentbuilder.EngineComponent{
		Engine:            picodataEngine,
		GlobalPriority:    spec.globalPriority,
		NodePriority:      spec.nodePriority,
		Dependencies:      dependencies,
		ConfigArtifactID:  configArtifactID(component.GetId(), component.GetRole()),
		ConfigFile:        configFile,
		ConfigOrigin:      configOrigin,
		DefaultConfigFile: picodataDefaultConfigFile(component, database, picodataWiring{}),
		InstallCommands:   picodataInstallCommands(component, dbPackage),
		ServiceFile:       picodataServiceFile(component.GetId(), component.GetRole(), configDir, wiring),
		Healthcheck:       picodataHealthcheckCommand(component.GetId()),
	}, nil
}

func picodataInstallCommands(component *topologypb.Component, dbPackage *domain.Package) []string {
	switch component.GetRole() {
	case picodataRoleInstance:
		if dbPackage == nil {
			return []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y picodata"}
		}
		commands := append([]string{}, dbPackage.GetPreInstall()...)
		if len(dbPackage.GetAptPackages()) > 0 {
			commands = append(commands, "DEBIAN_FRONTEND=noninteractive apt-get install -y "+strings.Join(dbPackage.GetAptPackages(), " "))
		}
		if dbPackage.GetDebFilename() != "" {
			commands = append(commands, "DEBIAN_FRONTEND=noninteractive apt-get install -y "+deploymentbuilder.ShellQuote(dbPackage.GetDebFilename()))
		}
		if len(commands) == 0 {
			commands = append(commands, "true")
		}
		return commands
	case picodataRoleHaproxy:
		return []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y haproxy"}
	default:
		return []string{"true"}
	}
}

func picodataServiceFile(componentID, role, configDir string, wiring picodataWiring) *common.File {
	return deploymentbuilder.EngineServiceFile(componentID, picodataServiceUnit(componentID, role, configDir, wiring))
}

func picodataServiceUnit(componentID, role, configDir string, wiring picodataWiring) string {
	cfgPath := picodataConfigPath(componentID, role)
	switch role {
	case picodataRoleInstance:
		dataDir := deploymentbuilder.DataDir(componentID)
		execStart := "/usr/bin/picodata run --config " + deploymentbuilder.ShellQuote(cfgPath)
		return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud Picodata instance %s
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
`,
			componentID,
			configDir,
			deploymentbuilder.ShellQuote(dataDir),
			execStart,
		)
	case picodataRoleHaproxy:
		return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, "/usr/sbin/haproxy -Ws -f "+deploymentbuilder.ShellQuote(cfgPath))
	default:
		return deploymentbuilder.SimpleServiceUnit(componentID, role, configDir, "/bin/false")
	}
}

func picodataEffectiveConfigFile(component *topologypb.Component, database *domain.Database, overrides *deploymentpb.RenderOverrideSet, wiring picodataWiring) (*common.File, deploymentpb.RenderArtifact_Origin) {
	artifactID := configArtifactID(component.GetId(), component.GetRole())
	if override, ok := deploymentbuilder.OverrideFile(overrides, component.GetId(), artifactID); ok && override.GetFile() != nil {
		return override.GetFile(), deploymentpb.RenderArtifact_ORIGIN_USER_OVERRIDE
	}
	return picodataDefaultConfigFile(component, database, wiring), deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT
}

func picodataDefaultConfigFile(component *topologypb.Component, database *domain.Database, wiring picodataWiring) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          picodataConfigPath(component.GetId(), component.GetRole()),
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: picodataConfigContentForRole(database, component.GetId(), component.GetRole(), wiring)},
	}
}

func picodataConfigContentForRole(database *domain.Database, componentID, role string, wiring picodataWiring) string {
	params := database.GetParams().GetPicodata()
	switch role {
	case picodataRoleHaproxy:
		return picodataHaproxyConfig(params.GetHaproxyOptions(), wiring.backends)
	default:
		return picodataConfigContent(componentID, params, wiring)
	}
}

// picodataHaproxyConfig renders an haproxy.cfg that load-balances the pgproto
// port across the resolved instance backends.
func picodataHaproxyConfig(options map[string]string, backends []string) string {
	var b strings.Builder
	b.WriteString("global\n  daemon\n")
	b.WriteString("defaults\n  mode tcp\n  timeout connect 5s\n  timeout client 1m\n  timeout server 1m\n")
	fmt.Fprintf(&b, "frontend pgproto\n  bind 0.0.0.0:%d\n  default_backend picodata\n", pgprotoPort)
	b.WriteString("backend picodata\n")
	for i, backend := range backends {
		fmt.Fprintf(&b, "  server instance-%d %s check\n", i+1, backend)
	}
	if extra := dbspec.RenderOptions(options, " "); extra != "" {
		b.WriteString(extra)
	}
	return b.String()
}

func picodataConfigPath(componentID, role string) string {
	return deploymentbuilder.ConfigDir(componentID) + "/" + picodataConfigFileName(role)
}

func picodataConfigFileName(role string) string {
	if role == picodataRoleHaproxy {
		return "haproxy.cfg"
	}
	return "picodata.yaml"
}

func configArtifactID(componentID, role string) string {
	return deploymentbuilder.ArtifactID(componentID, picodataConfigFileName(role))
}

func picodataHealthcheckCommand(componentID string) string {
	service := deploymentbuilder.ShellQuote(deploymentbuilder.ServiceName(componentID))
	return fmt.Sprintf(`for i in $(seq 1 60); do
  if systemctl is-active --quiet %s && curl -fsS http://127.0.0.1:%d/api/v1/health/ready >/dev/null; then
    exit 0
  fi
  sleep 2
done
echo "--- systemctl status %s ---"
systemctl status --no-pager -l %s || true
echo "--- journalctl %s ---"
journalctl --no-pager --output=short-iso-precise -u %s -n 200 || true
exit 1
`, service, httpPort, service, service, service, service)
}
