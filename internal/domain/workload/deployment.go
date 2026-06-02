package workload

import (
	"fmt"
	"path/filepath"
	"strings"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	"google.golang.org/protobuf/encoding/protojson"
)

type DeploymentRenderer struct{}

func (r DeploymentRenderer) Supports(component *topologypb.Component) bool {
	return component != nil && component.GetEngine() == Engine && component.GetRole() == RunnerRole
}

func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	if ctx.Workload == nil {
		return nil, fmt.Errorf("workload renderer requires workload input")
	}
	dependencies := deploymentbuilder.DependencyIDs(ctx, nil)
	configFile, _ := effectiveConfigFile(ctx.Component.GetId(), ctx.Workload, ctx.RenderOverrides)

	steps := []*deploymentpb.AgentStep{
		deploymentbuilder.CreateDirStep("010_create_config_dir", 10, deploymentbuilder.ConfigDir(ctx.Component.GetId()), 0755),
		deploymentbuilder.WriteFileStep("020_write_context", 20, deploymentbuilder.ContextFile(ctx, dependencies)),
		deploymentbuilder.WriteFileStep("030_write_config", 30, configFile),
	}
	steps = append(steps, workloadFileSteps(ctx.Component.GetId(), ctx.Workload)...)
	steps = append(steps,
		deploymentbuilder.CallCmdStep("100_prepare_stroppy", 100, installCommand(ctx.Workload)),
		deploymentbuilder.CallCmdStep("110_healthcheck", 110, "test -d "+deploymentbuilder.ShellQuote(deploymentbuilder.ConfigDir(ctx.Component.GetId()))),
	)

	return &deploymentpb.ComponentDeployment{
		ComponentId:           ctx.Component.GetId(),
		NodeId:                ctx.Node.GetId(),
		GlobalPriority:        90,
		NodePriority:          90,
		DependsOnComponentIds: dependencies,
		Steps:                 steps,
		Status:                common.Status_STATUS_PENDING,
		Labels: deploymentbuilder.MergeLabels(ctx.Component.GetLabels(), map[string]string{
			"engine": Engine,
			"role":   RunnerRole,
		}),
		Tags: ctx.Component.GetTags(),
	}, nil
}

func (r DeploymentRenderer) RenderPreview(ctx deploymentbuilder.PreviewContext) ([]*deploymentpb.RenderArtifact, error) {
	if ctx.Workload == nil {
		return nil, fmt.Errorf("workload renderer requires workload input")
	}
	dependencies := deploymentbuilder.DependencyIDs(deploymentbuilder.RenderContext{
		Topology:  ctx.Topology,
		Component: ctx.Component,
		Node:      ctx.Node,
	}, nil)
	configFile, configOrigin := effectiveConfigFile(ctx.Component.GetId(), ctx.Workload, ctx.RenderOverrides)
	defaultConfigFile := defaultConfigFile(ctx.Component.GetId(), ctx.Workload)

	artifacts := []*deploymentpb.RenderArtifact{
		deploymentbuilder.DirArtifact(ctx, Engine, "config-dir", &common.Dir{
			Info:          &common.Dir_Info{Path: deploymentbuilder.ConfigDir(ctx.Component.GetId()), Mode: 0755},
			CreateParents: true,
		}),
		deploymentbuilder.FileArtifact(ctx, Engine, deploymentbuilder.ArtifactID(ctx.Component.GetId(), "topology.env"), deploymentbuilder.PreviewContextFile(ctx, dependencies), deploymentpb.RenderArtifact_ORIGIN_SYSTEM, deploymentpb.RenderArtifact_MUTABILITY_READ_ONLY, "topology context is generated from topology and runtime state", "", map[string]string{"artifact": "context"}),
		deploymentbuilder.FileArtifact(ctx, Engine, configArtifactID(ctx.Component.GetId()), configFile, configOrigin, deploymentpb.RenderArtifact_MUTABILITY_EDITABLE, "", deploymentbuilder.FileHash(defaultConfigFile), map[string]string{"artifact": "config"}),
	}
	for _, file := range workloadFiles(ctx.Component.GetId(), ctx.Workload) {
		artifacts = append(artifacts, deploymentbuilder.FileArtifact(ctx, Engine, deploymentbuilder.ArtifactID(ctx.Component.GetId(), "files/"+filepath.Base(file.GetInfo().GetPath())), file, deploymentpb.RenderArtifact_ORIGIN_SYSTEM, deploymentpb.RenderArtifact_MUTABILITY_READ_ONLY, "workload files come from workload input", "", map[string]string{"artifact": "workload_file"}))
	}
	artifacts = append(artifacts,
		deploymentbuilder.CommandArtifact(ctx, Engine, deploymentbuilder.ArtifactID(ctx.Component.GetId(), "install/100"), installCommand(ctx.Workload), "stroppy install is renderer-owned", map[string]string{"artifact": "install"}),
		deploymentbuilder.CommandArtifact(ctx, Engine, deploymentbuilder.ArtifactID(ctx.Component.GetId(), "healthcheck"), "test -d "+deploymentbuilder.ShellQuote(deploymentbuilder.ConfigDir(ctx.Component.GetId())), "healthcheck command is renderer-owned", map[string]string{"artifact": "healthcheck"}),
		deploymentbuilder.RuntimeArtifact(ctx, Engine, "runtime/private-address", "machine."+ctx.Node.GetId()+".endpoint.private.address"),
	)
	return artifacts, nil
}

func effectiveConfigFile(componentID string, input *domain.Workload, overrides *deploymentpb.RenderOverrideSet) (*common.File, deploymentpb.RenderArtifact_Origin) {
	artifactID := configArtifactID(componentID)
	if override, ok := deploymentbuilder.OverrideFile(overrides, componentID, artifactID); ok && override.GetFile() != nil {
		return override.GetFile(), deploymentpb.RenderArtifact_ORIGIN_USER_OVERRIDE
	}
	return defaultConfigFile(componentID, input), deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT
}

func defaultConfigFile(componentID string, input *domain.Workload) *common.File {
	return &common.File{
		Info: &common.File_Info{
			Path:          configPath(componentID),
			Mode:          0644,
			CreateParents: true,
		},
		Content: &common.File_Text{Text: workloadConfig(input)},
	}
}

func workloadConfig(input *domain.Workload) string {
	data, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(input)
	if err != nil {
		return "{}\n"
	}
	return string(data) + "\n"
}

func workloadFileSteps(componentID string, input *domain.Workload) []*deploymentpb.AgentStep {
	files := workloadFiles(componentID, input)
	steps := make([]*deploymentpb.AgentStep, 0, len(files))
	order := uint32(40)
	for _, file := range files {
		steps = append(steps, deploymentbuilder.WriteFileStep(fmt.Sprintf("%03d_write_workload_file", order), order, file))
		order += 10
	}
	return steps
}

func workloadFiles(componentID string, input *domain.Workload) []*common.File {
	files := make([]*common.File, 0, len(input.GetFiles()))
	for _, file := range input.GetFiles() {
		files = append(files, &common.File{
			Info: &common.File_Info{
				Path:          deploymentbuilder.ConfigDir(componentID) + "/files/" + file.GetName(),
				Mode:          0644,
				CreateParents: true,
			},
			Content: &common.File_Text{Text: file.GetContent()},
		})
	}
	return files
}

func installCommand(input *domain.Workload) string {
	version := strings.TrimSpace(input.GetStroppyVersion())
	if version == "" {
		version = "latest"
	}
	return "mkdir -p /opt/stroppy && " +
		"if command -v stroppy >/dev/null 2>&1; then stroppy --version || true; else echo " + deploymentbuilder.ShellQuote("stroppy "+version+" binary resolver is pending") + "; fi"
}

func configPath(componentID string) string {
	return deploymentbuilder.ConfigDir(componentID) + "/stroppy-config.json"
}

func configArtifactID(componentID string) string {
	return deploymentbuilder.ArtifactID(componentID, "stroppy-config.json")
}
