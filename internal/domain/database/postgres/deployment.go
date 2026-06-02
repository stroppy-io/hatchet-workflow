package postgres

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

type DeploymentRenderer struct{}

func (r DeploymentRenderer) Supports(component *topologypb.Component) bool {
	return component != nil && component.GetEngine() == postgresEngine
}

func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	spec, ok := postgresDeploymentSpecs[ctx.Component.GetRole()]
	if !ok {
		return nil, fmt.Errorf("unsupported postgres component role %q", ctx.Component.GetRole())
	}

	dependencies := postgresDependencies(ctx)
	configFile, configOrigin := postgresEffectiveConfigFile(ctx.Component, ctx.Database, ctx.RenderOverrides)
	installCommands := postgresInstallCommands(ctx.Component, ctx.DatabasePackage)
	rendered := &deploymentpb.ComponentDeployment{
		ComponentId:           ctx.Component.GetId(),
		NodeId:                ctx.Node.GetId(),
		GlobalPriority:        spec.globalPriority,
		NodePriority:          spec.nodePriority,
		DependsOnComponentIds: dependencies,
		Steps:                 make([]*deploymentpb.AgentStep, 0, 8+len(installCommands)),
		Status:                common.Status_STATUS_PENDING,
		Labels: deploymentbuilder.MergeLabels(ctx.Component.GetLabels(), map[string]string{
			"engine": ctx.Component.GetEngine(),
			"role":   ctx.Component.GetRole(),
		}),
		Tags: ctx.Component.GetTags(),
	}

	configDir := deploymentbuilder.ConfigDir(ctx.Component.GetId())
	rendered.Steps = append(rendered.Steps,
		deploymentbuilder.CreateDirStep("010_create_config_dir", 10, configDir, 0755),
		deploymentbuilder.WriteFileStep("020_write_context", 20, deploymentbuilder.ContextFile(ctx, dependencies)),
	)

	configStep := deploymentbuilder.WriteFileStep("030_write_config", 30, configFile)
	configKind := "config_default"
	if configOrigin == deploymentpb.RenderArtifact_ORIGIN_USER_OVERRIDE {
		configKind = "config_override"
	}
	configStep.Labels = map[string]string{
		"kind":        configKind,
		"artifact_id": configArtifactID(ctx.Component.GetId(), ctx.Component.GetRole()),
	}
	configStep.Tags = deploymentbuilder.Tags("deployment", "config")
	rendered.Steps = append(rendered.Steps, configStep)

	installOrder := uint32(100)
	for _, command := range installCommands {
		rendered.Steps = append(rendered.Steps, deploymentbuilder.CallCmdStep(fmt.Sprintf("%03d_install", installOrder), installOrder, command))
		installOrder += 10
	}

	rendered.Steps = append(rendered.Steps,
		deploymentbuilder.WriteFileStep("200_write_service", 200, deploymentbuilder.ServiceFile(ctx.Component.GetId(), ctx.Component.GetRole(), configDir)),
		deploymentbuilder.CallCmdStep("210_reload_systemd", 210, "systemctl daemon-reload"),
		deploymentbuilder.CallCmdStep("220_enable_start", 220, fmt.Sprintf("systemctl enable --now %s", deploymentbuilder.ServiceName(ctx.Component.GetId()))),
		deploymentbuilder.CallCmdStep("230_healthcheck", 230, spec.healthcheckCommand),
	)

	return rendered, nil
}

func (r DeploymentRenderer) RenderPreview(ctx deploymentbuilder.PreviewContext) ([]*deploymentpb.RenderArtifact, error) {
	spec, ok := postgresDeploymentSpecs[ctx.Component.GetRole()]
	if !ok {
		return nil, fmt.Errorf("unsupported postgres component role %q", ctx.Component.GetRole())
	}

	dependencies := postgresPreviewDependencies(ctx)
	configDir := deploymentbuilder.ConfigDir(ctx.Component.GetId())
	configFile, configOrigin := postgresEffectiveConfigFile(ctx.Component, ctx.Database, ctx.RenderOverrides)
	defaultConfigFile := postgresDefaultConfigFile(ctx.Component, ctx.Database)
	configID := configArtifactID(ctx.Component.GetId(), ctx.Component.GetRole())

	artifacts := []*deploymentpb.RenderArtifact{
		postgresDirArtifact(ctx, "config-dir", &common.Dir{
			Info: &common.Dir_Info{
				Path: configDir,
				Mode: 0755,
			},
			CreateParents: true,
		}),
		postgresFileArtifact(
			ctx,
			deploymentbuilder.ArtifactID(ctx.Component.GetId(), "topology.env"),
			postgresPreviewContextFile(ctx, dependencies),
			deploymentpb.RenderArtifact_ORIGIN_SYSTEM,
			deploymentpb.RenderArtifact_MUTABILITY_READ_ONLY,
			"topology context is generated from topology and runtime state",
			"",
			map[string]string{"artifact": "context"},
		),
		postgresFileArtifact(
			ctx,
			configID,
			configFile,
			configOrigin,
			deploymentpb.RenderArtifact_MUTABILITY_EDITABLE,
			"",
			deploymentbuilder.FileHash(defaultConfigFile),
			map[string]string{"artifact": "config"},
		),
		postgresFileArtifact(
			ctx,
			deploymentbuilder.ArtifactID(ctx.Component.GetId(), "systemd.service"),
			deploymentbuilder.ServiceFile(ctx.Component.GetId(), ctx.Component.GetRole(), configDir),
			deploymentpb.RenderArtifact_ORIGIN_SYSTEM,
			deploymentpb.RenderArtifact_MUTABILITY_READ_ONLY,
			"service unit is generated by the deployment renderer",
			"",
			map[string]string{"artifact": "service"},
		),
		postgresRuntimeArtifact(ctx, "runtime/private-address", "machine."+ctx.Node.GetId()+".endpoint.private.address"),
	}

	installOrder := uint32(100)
	for _, command := range postgresInstallCommands(ctx.Component, ctx.DatabasePackage) {
		artifacts = append(artifacts, postgresCommandArtifact(
			ctx,
			deploymentbuilder.ArtifactID(ctx.Component.GetId(), fmt.Sprintf("install/%03d", installOrder)),
			command,
			"package installation command is renderer-owned",
			map[string]string{"artifact": "install"},
		))
		installOrder += 10
	}
	artifacts = append(artifacts,
		postgresCommandArtifact(ctx, deploymentbuilder.ArtifactID(ctx.Component.GetId(), "systemd/reload"), "systemctl daemon-reload", "systemd reload is renderer-owned", map[string]string{"artifact": "systemd_reload"}),
		postgresCommandArtifact(ctx, deploymentbuilder.ArtifactID(ctx.Component.GetId(), "systemd/enable-start"), fmt.Sprintf("systemctl enable --now %s", deploymentbuilder.ServiceName(ctx.Component.GetId())), "systemd activation is renderer-owned", map[string]string{"artifact": "systemd_enable_start"}),
		postgresCommandArtifact(ctx, deploymentbuilder.ArtifactID(ctx.Component.GetId(), "healthcheck"), spec.healthcheckCommand, "healthcheck command is renderer-owned", map[string]string{"artifact": "healthcheck"}),
	)

	return artifacts, nil
}

type postgresDeploymentSpec struct {
	globalPriority     uint32
	nodePriority       uint32
	healthcheckCommand string
}

var postgresDeploymentSpecs = map[string]postgresDeploymentSpec{
	postgresRoleEtcd:      {globalPriority: 10, nodePriority: 10, healthcheckCommand: "command -v etcd >/dev/null"},
	postgresRoleMaster:    {globalPriority: 20, nodePriority: 20, healthcheckCommand: "command -v psql >/dev/null"},
	postgresRoleReplica:   {globalPriority: 30, nodePriority: 20, healthcheckCommand: "command -v psql >/dev/null"},
	postgresRolePatroni:   {globalPriority: 40, nodePriority: 30, healthcheckCommand: "command -v patroni >/dev/null"},
	postgresRolePgbouncer: {globalPriority: 50, nodePriority: 40, healthcheckCommand: "command -v pgbouncer >/dev/null"},
	postgresRoleHaproxy:   {globalPriority: 60, nodePriority: 50, healthcheckCommand: "command -v haproxy >/dev/null"},
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
		if dbPackage == nil {
			return []string{"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y postgresql postgresql-contrib"}
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

func postgresEffectiveConfigFile(component *topologypb.Component, database *domain.Database, overrides *deploymentpb.RenderOverrideSet) (*common.File, deploymentpb.RenderArtifact_Origin) {
	artifactID := configArtifactID(component.GetId(), component.GetRole())
	if override, ok := deploymentbuilder.OverrideFile(overrides, component.GetId(), artifactID); ok && override.GetFile() != nil {
		return override.GetFile(), deploymentpb.RenderArtifact_ORIGIN_USER_OVERRIDE
	}
	return postgresDefaultConfigFile(component, database), deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT
}

func postgresDefaultConfigFile(component *topologypb.Component, database *domain.Database) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          configPath(component.GetId(), component.GetRole()),
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: configContent(component.GetRole(), postgresRoleOptions(database, component.GetRole()))},
	}
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

func postgresPreviewContextFile(ctx deploymentbuilder.PreviewContext, dependencies []string) *common.File {
	var b strings.Builder
	fmt.Fprintf(&b, "COMPONENT_ID=%s\n", deploymentbuilder.ShellValue(ctx.Component.GetId()))
	fmt.Fprintf(&b, "NODE_ID=%s\n", deploymentbuilder.ShellValue(ctx.Node.GetId()))
	fmt.Fprintf(&b, "ENGINE=%s\n", deploymentbuilder.ShellValue(ctx.Component.GetEngine()))
	fmt.Fprintf(&b, "ROLE=%s\n", deploymentbuilder.ShellValue(ctx.Component.GetRole()))
	if len(dependencies) > 0 {
		fmt.Fprintf(&b, "DEPENDS_ON=%s\n", deploymentbuilder.ShellValue(strings.Join(dependencies, ",")))
	}
	fmt.Fprintf(&b, "ENDPOINT_PRIVATE_ADDRESS=%s\n", deploymentbuilder.ShellValue("${runtime.machine."+ctx.Node.GetId()+".endpoint.private.address}"))

	return &common.File{
		Info: &common.File_Info{
			Path:          deploymentbuilder.ConfigDir(ctx.Component.GetId()) + "/topology.env",
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: b.String()},
	}
}

func postgresFileArtifact(
	ctx deploymentbuilder.PreviewContext,
	artifactID string,
	file *common.File,
	origin deploymentpb.RenderArtifact_Origin,
	mutability deploymentpb.RenderArtifact_Mutability,
	lockReason string,
	baseHash string,
	labels map[string]string,
) *deploymentpb.RenderArtifact {
	return &deploymentpb.RenderArtifact{
		Id:           artifactID,
		ComponentId:  ctx.Component.GetId(),
		Kind:         deploymentpb.RenderArtifact_KIND_FILE,
		Origin:       origin,
		Mutability:   mutability,
		LockReason:   lockReason,
		Artifact:     &deploymentpb.RenderArtifact_File{File: file},
		RendererName: "postgres",
		BaseHash:     baseHash,
		Labels: deploymentbuilder.MergeLabels(map[string]string{
			"engine": postgresEngine,
			"role":   ctx.Component.GetRole(),
		}, labels),
		Tags: deploymentbuilder.Tags("render", "postgres", "file"),
	}
}

func postgresCommandArtifact(ctx deploymentbuilder.PreviewContext, artifactID, script, lockReason string, labels map[string]string) *deploymentpb.RenderArtifact {
	return &deploymentpb.RenderArtifact{
		Id:           artifactID,
		ComponentId:  ctx.Component.GetId(),
		Kind:         deploymentpb.RenderArtifact_KIND_COMMAND,
		Origin:       deploymentpb.RenderArtifact_ORIGIN_SYSTEM,
		Mutability:   deploymentpb.RenderArtifact_MUTABILITY_READ_ONLY,
		LockReason:   lockReason,
		Artifact:     &deploymentpb.RenderArtifact_Cmd{Cmd: deploymentbuilder.ShellCmd(script)},
		RendererName: "postgres",
		Labels: deploymentbuilder.MergeLabels(map[string]string{
			"engine": postgresEngine,
			"role":   ctx.Component.GetRole(),
		}, labels),
		Tags: deploymentbuilder.Tags("render", "postgres", "command"),
	}
}

func postgresDirArtifact(ctx deploymentbuilder.PreviewContext, name string, dir *common.Dir) *deploymentpb.RenderArtifact {
	return &deploymentpb.RenderArtifact{
		Id:           deploymentbuilder.ArtifactID(ctx.Component.GetId(), name),
		ComponentId:  ctx.Component.GetId(),
		Kind:         deploymentpb.RenderArtifact_KIND_DIRECTORY,
		Origin:       deploymentpb.RenderArtifact_ORIGIN_SYSTEM,
		Mutability:   deploymentpb.RenderArtifact_MUTABILITY_READ_ONLY,
		LockReason:   "filesystem layout is renderer-owned",
		Artifact:     &deploymentpb.RenderArtifact_Dir{Dir: dir},
		RendererName: "postgres",
		Labels: map[string]string{
			"engine":   postgresEngine,
			"role":     ctx.Component.GetRole(),
			"artifact": "directory",
		},
		Tags: deploymentbuilder.Tags("render", "postgres", "directory"),
	}
}

func postgresRuntimeArtifact(ctx deploymentbuilder.PreviewContext, name, runtimeValue string) *deploymentpb.RenderArtifact {
	return &deploymentpb.RenderArtifact{
		Id:           deploymentbuilder.ArtifactID(ctx.Component.GetId(), name),
		ComponentId:  ctx.Component.GetId(),
		Kind:         deploymentpb.RenderArtifact_KIND_RUNTIME_VALUE,
		Origin:       deploymentpb.RenderArtifact_ORIGIN_RUNTIME,
		Mutability:   deploymentpb.RenderArtifact_MUTABILITY_RUNTIME_ONLY,
		LockReason:   "resolved after infrastructure provisioning",
		Artifact:     &deploymentpb.RenderArtifact_RuntimeValue{RuntimeValue: runtimeValue},
		RendererName: "postgres",
		Labels: map[string]string{
			"engine":   postgresEngine,
			"role":     ctx.Component.GetRole(),
			"artifact": "runtime_value",
		},
		Tags: deploymentbuilder.Tags("render", "postgres", "runtime"),
	}
}

func configArtifactID(componentID, role string) string {
	return deploymentbuilder.ArtifactID(componentID, configFileName(role))
}
