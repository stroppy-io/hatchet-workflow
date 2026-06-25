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
	configFile := orioledbDefaultConfigFile(configDir)

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
		Healthcheck:       "for i in $(seq 1 60); do docker exec " + containerName + " pg_isready -h 127.0.0.1 -p " + fmt.Sprint(pgPort) + " && exit 0; sleep 3; done; exit 1",
	}, nil
}

// orioledbDefaultConfigFile returns the env file written to the config dir.
// Trust auth (not a password) mirrors the native-Postgres deploy: the stroppy
// workload and the local postgres_exporter both connect passwordless over
// 127.0.0.1, so the container must accept unauthenticated local/TCP connections
// exactly like native pg. The EnvironmentFile in the systemd unit loads it.
func orioledbDefaultConfigFile(configDir string) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          configDir + "/orioledb.env",
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: "POSTGRES_HOST_AUTH_METHOD=trust\n"},
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

// orioledbInstallCommands installs docker.io then configures + starts dockerd.
// dbPackage is always set for builtin orioledb.
func orioledbInstallCommands(dbPackage *domain.Package) []string {
	var commands []string
	if dbPackage != nil {
		commands = append(commands, dbPackage.GetPreInstall()...)
		if pkgs := dbPackage.GetAptPackages(); len(pkgs) > 0 {
			// Refresh the index first: the cloud-init apt update goes stale by
			// deploy time, so apt asks the apt-cacher for .deb versions the mirror
			// has already superseded -> 404. `apt-get update` realigns the index
			// with what the mirror (via apt-cacher) actually serves.
			commands = append(commands,
				"apt-get update",
				"DEBIAN_FRONTEND=noninteractive apt-get install -y "+strings.Join(pkgs, " "))
		}
	}
	// Configure the registry mirror + start dockerd AFTER docker.io is installed
	// (docker.service does not exist before the apt install).
	commands = append(commands, dockerDaemonSetupCommands()...)
	return commands
}

// orioledbServiceUnit renders a systemd unit that pulls and runs the OrioleDB
// container with benchmark-faithful flags so the container does not skew the
// numbers vs bare metal: --network host (no NAT, 5432 directly on the node),
// --pid/--ipc host (no namespace isolation; shared memory like a host process),
// --privileged (direct/async IO, huge pages, sysctl access the engine may use),
// and a bind mount of the host data dir to PGDATA (writes hit the real disk, not
// docker's overlay/CoW layer). Trust auth is loaded from the EnvironmentFile.
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
ExecStart=/usr/bin/docker run --rm --name %s --network host --pid host --ipc host --privileged \
  -e POSTGRES_HOST_AUTH_METHOD \
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
