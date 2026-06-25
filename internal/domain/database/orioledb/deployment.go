package orioledb

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

const containerName = "stroppy-orioledb"

// DeploymentRenderer implements the deployment renderer for OrioleDB.
type DeploymentRenderer struct{}

func (r DeploymentRenderer) Supports(component *topologypb.Component) bool {
	return component != nil && component.GetEngine() == orioledbEngine
}

func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	ec, err := orioledbEngineComponent(ctx.Component, ctx.Database, ctx.DatabasePackage, deploymentbuilder.DependencyIDs(ctx, nil))
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
	ec, err := orioledbEngineComponent(ctx.Component, ctx.Database, ctx.DatabasePackage, deps)
	if err != nil {
		return nil, err
	}
	return deploymentbuilder.RenderComponentPreview(ctx, ec), nil
}

func orioledbEngineComponent(
	component *topologypb.Component,
	database *domain.Database,
	dbPackage *domain.Package,
	dependencies []string,
) (deploymentbuilder.EngineComponent, error) {
	if component.GetRole() != orioledbRoleMaster {
		return deploymentbuilder.EngineComponent{}, fmt.Errorf("unsupported orioledb component role %q", component.GetRole())
	}
	params := database.GetParams().GetOrioledb()
	image := params.GetImage()
	if image == "" {
		image = defaultImage
	}
	locale := params.GetInitdbLocale()
	if locale == "" {
		locale = "C"
	}
	options := pgOptions(params)
	configDir := deploymentbuilder.ConfigDir(component.GetId())
	configFile := orioledbDefaultConfigFile(component.GetId(), configDir)

	return deploymentbuilder.EngineComponent{
		Engine:            orioledbEngine,
		GlobalPriority:    20,
		NodePriority:      20,
		Dependencies:      dependencies,
		ConfigArtifactID:  deploymentbuilder.ArtifactID(component.GetId(), "orioledb.env"),
		ConfigFile:        configFile,
		ConfigOrigin:      deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT,
		DefaultConfigFile: configFile,
		InstallCommands:   orioledbInstallCommands(dbPackage),
		ServiceFile:       deploymentbuilder.EngineServiceFile(component.GetId(), orioledbServiceUnit(component.GetId(), image, locale, options)),
		Healthcheck:       "docker exec " + containerName + " pg_isready -h 127.0.0.1 -p " + fmt.Sprint(pgPort),
	}, nil
}

// orioledbDefaultConfigFile returns the env file written to the config dir.
// The EnvironmentFile in the systemd unit loads POSTGRES_PASSWORD from here.
func orioledbDefaultConfigFile(componentID, configDir string) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          configDir + "/orioledb.env",
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: "POSTGRES_PASSWORD=stroppy\n"},
	}
}

// pgOptions merges the explicit shared_buffers_mb knob into postgres_options.
func pgOptions(params *domain.OrioledbParams) map[string]string {
	out := map[string]string{}
	for k, v := range params.GetPostgresOptions() {
		out[k] = v
	}
	if mb := params.GetSharedBuffersMb(); mb > 0 {
		out["shared_buffers"] = fmt.Sprintf("%dMB", mb)
	}
	return out
}

// orioledbInstallCommands runs the package pre_install (docker + mirror) then
// apt install of docker.io. dbPackage is always set for builtin orioledb.
func orioledbInstallCommands(dbPackage *domain.Package) []string {
	var commands []string
	if dbPackage != nil {
		commands = append(commands, dbPackage.GetPreInstall()...)
		if pkgs := dbPackage.GetAptPackages(); len(pkgs) > 0 {
			commands = append(commands,
				"DEBIAN_FRONTEND=noninteractive apt-get install -y "+strings.Join(pkgs, " "))
		}
	}
	return commands
}

// orioledbServiceUnit renders a systemd unit that pulls and runs the OrioleDB
// container with host networking (so port 5432 is on the node exactly as native
// Postgres). POSTGRES_PASSWORD is loaded from the EnvironmentFile.
func orioledbServiceUnit(componentID, image, locale string, options map[string]string) string {
	var optStr strings.Builder
	keys := make([]string, 0, len(options))
	for k := range options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&optStr, " -c %s=%s", k, options[k])
	}
	dataDir := deploymentbuilder.DataDir(componentID)
	configDir := deploymentbuilder.ConfigDir(componentID)
	return fmt.Sprintf(`[Unit]
Description=Stroppy Cloud OrioleDB %s
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
EnvironmentFile=%s/orioledb.env
ExecStartPre=-/usr/bin/docker rm -f %s
ExecStartPre=/usr/bin/docker pull %s
ExecStartPre=/bin/mkdir -p %s
ExecStart=/usr/bin/docker run --rm --name %s --network host \
  -e POSTGRES_PASSWORD \
  -e POSTGRES_INITDB_ARGS=--locale=%s \
  -v %s:/var/lib/postgresql/data \
  %s%s
ExecStop=/usr/bin/docker rm -f %s
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`,
		componentID,
		configDir,
		containerName,
		image,
		deploymentbuilder.ShellQuote(dataDir),
		containerName,
		locale,
		deploymentbuilder.ShellQuote(dataDir),
		image,
		optStr.String(),
		containerName,
	)
}
