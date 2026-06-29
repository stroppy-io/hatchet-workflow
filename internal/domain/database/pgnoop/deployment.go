package pgnoop

import (
	"fmt"
	"sort"
	"strings"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

// defaultDownloadURL is the stock pg-noop tarball used when no package override
// supplies one (binary cache route; falls back to the pinned default version).
var defaultDownloadURL = serverBinaryURL("pgnoop", defaultPgNoopVersion, downloadAsset)

type DeploymentRenderer struct{}

func (r DeploymentRenderer) Supports(component *topologypb.Component) bool {
	return component != nil && component.GetEngine() == pgnoopEngine
}

func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	ec, err := pgnoopEngineComponent(ctx.Component, ctx.Database, ctx.DatabasePackage, deploymentbuilder.DependencyIDs(ctx, nil))
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
	ec, err := pgnoopEngineComponent(ctx.Component, ctx.Database, ctx.DatabasePackage, deps)
	if err != nil {
		return nil, err
	}
	return deploymentbuilder.RenderComponentPreview(ctx, ec), nil
}

func pgnoopEngineComponent(
	component *topologypb.Component,
	database *domain.Database,
	dbPackage *domain.Package,
	dependencies []string,
) (deploymentbuilder.EngineComponent, error) {
	if component.GetRole() != pgnoopRoleNode {
		return deploymentbuilder.EngineComponent{}, fmt.Errorf("unsupported pgnoop component role %q", component.GetRole())
	}

	configDir := deploymentbuilder.ConfigDir(component.GetId())
	configFile := pgnoopEnvFile(configDir, database.GetParams().GetPgNoop())

	return deploymentbuilder.EngineComponent{
		Engine:            pgnoopEngine,
		GlobalPriority:    20,
		NodePriority:      20,
		Dependencies:      dependencies,
		ConfigArtifactID:  deploymentbuilder.ArtifactID(component.GetId(), "pgnoop.env"),
		ConfigFile:        configFile,
		ConfigOrigin:      deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT,
		DefaultConfigFile: configFile,
		InstallCommands:   pgnoopInstallCommands(dbPackage),
		ServiceFile:       deploymentbuilder.EngineServiceFile(component.GetId(), pgnoopServiceUnit(component.GetId(), configDir)),
		Healthcheck:       "systemctl is-active --quiet " + deploymentbuilder.ShellQuote(deploymentbuilder.ServiceName(component.GetId())),
	}, nil
}

// pgnoopEnvFile renders the EnvironmentFile pg-noop reads (PGNOOP_*). The blackhole
// listens on every interface so the runner can reach it; workers default to the
// binary's one-per-CPU when unset.
func pgnoopEnvFile(configDir string, params *domain.PgNoopParams) *common.File {
	var b strings.Builder
	b.WriteString("PGNOOP_HOST=0.0.0.0\n")
	fmt.Fprintf(&b, "PGNOOP_PORT=%d\n", pgPort)
	if workers := params.GetWorkers(); workers > 0 {
		fmt.Fprintf(&b, "PGNOOP_WORKERS=%d\n", workers)
	}
	keys := make([]string, 0, len(params.GetOptions()))
	for k := range params.GetOptions() {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, params.GetOptions()[k])
	}
	return &common.File{
		Info: &common.File_Info{
			Path:          configDir + "/pgnoop.env",
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: b.String()},
	}
}

// pgnoopInstallCommands downloads the static pg-noop tarball and installs the
// binary to /usr/local/bin/pgnoop. The release archive name varies (pgnoop /
// pg-noop), so the binary is located by find rather than assumed.
func pgnoopInstallCommands(dbPackage *domain.Package) []string {
	url := defaultDownloadURL
	var commands []string
	if dbPackage != nil {
		commands = append(commands, dbPackage.GetPreInstall()...)
		if dbPackage.GetDebFilename() != "" {
			url = dbPackage.GetDebFilename()
		}
	}
	commands = append(commands,
		deploymentbuilder.CurlDownloadCommand(url, "/tmp/pgnoop.tar.xz")+
			` && mkdir -p /opt/pgnoop && tar -xJf /tmp/pgnoop.tar.xz -C /opt/pgnoop`+
			` && install -m 0755 "$(find /opt/pgnoop -type f \( -name pgnoop -o -name pg-noop \) | head -n1)" /usr/local/bin/pgnoop`,
	)
	return commands
}

func pgnoopServiceUnit(componentID, configDir string) string {
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud pg-noop blackhole %s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=%s/pgnoop.env
ExecStart=/usr/local/bin/pgnoop
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
`, componentID, configDir)
}
