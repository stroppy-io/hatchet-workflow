package cockroach

import (
	"fmt"
	"strings"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

// defaultCockroachDownloadURL is the stock cockroach release tarball used when
// no package override supplies one. CockroachDB ships a binary, not an apt
// package.
const defaultCockroachDownloadURL = "${STROPPY_SERVER_ADDR%/}/api/binaries/cockroach/23.2.5/cockroach-v23.2.5.linux-amd64.tgz"

type DeploymentRenderer struct{}

func (r DeploymentRenderer) Supports(component *topologypb.Component) bool {
	return component != nil && component.GetEngine() == cockroachEngine
}

func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	wiring, err := cockroachResolveWiring(ctx)
	if err != nil {
		return nil, err
	}
	ec, err := cockroachEngineComponent(ctx.Component, ctx.Database, ctx.RenderOverrides, ctx.DatabasePackage, deploymentbuilder.DependencyIDs(ctx, nil), wiring)
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
	// Preview has no runtime addresses (infrastructure is not provisioned yet),
	// so the unit is rendered without resolved peers.
	ec, err := cockroachEngineComponent(ctx.Component, ctx.Database, ctx.RenderOverrides, ctx.DatabasePackage, deps, cockroachWiring{})
	if err != nil {
		return nil, err
	}
	return deploymentbuilder.RenderComponentPreview(ctx, ec), nil
}

// cockroachWiring carries the runtime-resolved cluster topology for one node.
type cockroachWiring struct {
	advertise string   // own advertise host:port
	joinAddrs []string // every node's rpc host:port (the --join set)
	isInit    bool     // first node performs `cockroach init`
}

func cockroachResolveWiring(ctx deploymentbuilder.RenderContext) (cockroachWiring, error) {
	own, ok := deploymentbuilder.OwnPrivateEndpoint(ctx)
	if !ok {
		return cockroachWiring{}, fmt.Errorf("cockroach component %q has no private endpoint", ctx.Component.GetId())
	}
	nodes, err := deploymentbuilder.ComponentTargets(ctx, func(c *topologypb.Component) bool {
		return c.GetEngine() == cockroachEngine && c.GetRole() == cockroachRoleNode
	}, "rpc", sqlPort)
	if err != nil {
		return cockroachWiring{}, err
	}

	addrs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		addrs = append(addrs, node.AddressPort())
	}
	return cockroachWiring{
		advertise: deploymentbuilder.AddressPort(own.Address, sqlPort),
		joinAddrs: addrs,
		isInit:    len(nodes) > 0 && nodes[0].ComponentID == ctx.Component.GetId(),
	}, nil
}

func cockroachEngineComponent(
	component *topologypb.Component,
	database *domain.Database,
	overrides *deploymentpb.RenderOverrideSet,
	dbPackage *domain.Package,
	dependencies []string,
	wiring cockroachWiring,
) (deploymentbuilder.EngineComponent, error) {
	if component.GetRole() != cockroachRoleNode {
		return deploymentbuilder.EngineComponent{}, fmt.Errorf("unsupported cockroach component role %q", component.GetRole())
	}

	configFile, configOrigin := cockroachEffectiveConfigFile(component, database, overrides)
	configDir := deploymentbuilder.ConfigDir(component.GetId())

	ec := deploymentbuilder.EngineComponent{
		Engine:            cockroachEngine,
		GlobalPriority:    20,
		NodePriority:      20,
		Dependencies:      dependencies,
		ConfigArtifactID:  configArtifactID(component.GetId()),
		ConfigFile:        configFile,
		ConfigOrigin:      configOrigin,
		DefaultConfigFile: cockroachDefaultConfigFile(component, database),
		InstallCommands:   cockroachInstallCommands(dbPackage),
		ServiceFile:       cockroachServiceFile(component.GetId(), configDir, wiring),
		Healthcheck:       "systemctl is-active --quiet " + deploymentbuilder.ShellQuote(deploymentbuilder.ServiceName(component.GetId())),
	}

	if settings := cockroachClusterSettings(database.GetParams().GetCockroach().GetOptions()); settings != "" {
		ec.ExtraFiles = []deploymentbuilder.EngineFile{{
			StepID:        "040_write_cluster_settings",
			StepOrder:     40,
			ArtifactName:  "cluster-settings.sql",
			ArtifactLabel: "cluster_settings",
			File: &common.File{
				Info: &common.File_Info{
					Path:          deploymentbuilder.ConfigDir(component.GetId()) + "/cluster-settings.sql",
					Mode:          0644,
					CreateParents: true,
				},
				Content: &common.File_Text{Text: settings},
			},
		}}
	}
	return ec, nil
}

func cockroachInstallCommands(dbPackage *domain.Package) []string {
	url := defaultCockroachDownloadURL
	var commands []string
	if dbPackage != nil {
		commands = append(commands, dbPackage.GetPreInstall()...)
		if dbPackage.GetDebFilename() != "" {
			url = dbPackage.GetDebFilename()
		}
	}
	commands = append(commands,
		"curl -fsSL "+deploymentbuilder.ShellQuoteServerAddrURL(url)+" -o /tmp/cockroach.tgz && tar -xzf /tmp/cockroach.tgz -C /opt && install -m 0755 \"$(find /opt -name cockroach -type f | head -n1)\" /usr/local/bin/cockroach",
	)
	return commands
}

func cockroachServiceFile(componentID, configDir string, wiring cockroachWiring) *common.File {
	return deploymentbuilder.EngineServiceFile(componentID, cockroachServiceUnit(componentID, configDir, wiring))
}

func cockroachServiceUnit(componentID, configDir string, wiring cockroachWiring) string {
	dataDir := deploymentbuilder.DataDir(componentID)
	flagsPath := cockroachConfigPath(componentID)

	// Without resolved peers (preview), fall back to a standalone single node.
	if len(wiring.joinAddrs) == 0 {
		return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud CockroachDB node %s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=%s/topology.env
ExecStartPre=/bin/mkdir -p %s
ExecStart=/bin/sh -ec "exec /usr/local/bin/cockroach start-single-node --insecure --store=%s --listen-addr=0.0.0.0:%d --http-addr=0.0.0.0:%d $$(grep -v '^#' %s 2>/dev/null | tr '\n' ' ')"
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`,
			componentID,
			configDir,
			deploymentbuilder.ShellQuote(dataDir),
			deploymentbuilder.ShellQuote(dataDir),
			sqlPort,
			httpPort,
			deploymentbuilder.ShellQuote(flagsPath),
		)
	}

	join := strings.Join(wiring.joinAddrs, ",")
	initLine := ""
	if wiring.isInit {
		initLine = fmt.Sprintf("ExecStartPost=/bin/sh -c '/usr/local/bin/cockroach init --insecure --host=%s || true'\n", deploymentbuilder.ShellQuote(wiring.advertise))
	}
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud CockroachDB node %s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=%s/topology.env
ExecStartPre=/bin/mkdir -p %s
ExecStart=/bin/sh -ec "exec /usr/local/bin/cockroach start --insecure --store=%s --listen-addr=0.0.0.0:%d --advertise-addr=%s --http-addr=0.0.0.0:%d --join=%s $$(grep -v '^#' %s 2>/dev/null | tr '\n' ' ')"
%sRestart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`,
		componentID,
		configDir,
		deploymentbuilder.ShellQuote(dataDir),
		deploymentbuilder.ShellQuote(dataDir),
		sqlPort,
		deploymentbuilder.ShellQuote(wiring.advertise),
		httpPort,
		deploymentbuilder.ShellQuote(join),
		deploymentbuilder.ShellQuote(flagsPath),
		initLine,
	)
}

func cockroachEffectiveConfigFile(component *topologypb.Component, database *domain.Database, overrides *deploymentpb.RenderOverrideSet) (*common.File, deploymentpb.RenderArtifact_Origin) {
	artifactID := configArtifactID(component.GetId())
	if override, ok := deploymentbuilder.OverrideFile(overrides, component.GetId(), artifactID); ok && override.GetFile() != nil {
		return override.GetFile(), deploymentpb.RenderArtifact_ORIGIN_USER_OVERRIDE
	}
	return cockroachDefaultConfigFile(component, database), deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT
}

func cockroachDefaultConfigFile(component *topologypb.Component, database *domain.Database) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          cockroachConfigPath(component.GetId()),
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: cockroachFlags(database.GetParams().GetCockroach().GetOptions())},
	}
}

func cockroachConfigPath(componentID string) string {
	return deploymentbuilder.ConfigDir(componentID) + "/cockroach.flags"
}

func configArtifactID(componentID string) string {
	return deploymentbuilder.ArtifactID(componentID, "cockroach.flags")
}
