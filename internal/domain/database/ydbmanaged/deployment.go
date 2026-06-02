package ydbmanaged

import (
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

// DeploymentRenderer renders the managed-YDB database component. Managed YDB is
// provisioned by the cloud provider (terraform managed_ydb block), so there is
// no agent install/configure/start work to perform on the node — the renderer
// emits a single no-op marker step so the component participates in the plan
// (and its node/endpoint is resolvable for the workload connection) without
// running anything.
type DeploymentRenderer struct{}

func (r DeploymentRenderer) Supports(component *topologypb.Component) bool {
	return component != nil && component.GetEngine() == Engine && component.GetRole() == Role
}

func (r DeploymentRenderer) RenderComponent(ctx deploymentbuilder.RenderContext) (*deploymentpb.ComponentDeployment, error) {
	steps := []*deploymentpb.AgentStep{
		deploymentbuilder.CallCmdStep("010_managed_marker", 10,
			"echo 'managed YDB is provider-managed; nothing to install on this node'"),
	}

	return &deploymentpb.ComponentDeployment{
		ComponentId:    ctx.Component.GetId(),
		NodeId:         ctx.Node.GetId(),
		GlobalPriority: 10,
		NodePriority:   10,
		Steps:          steps,
		Status:         common.Status_STATUS_PENDING,
		Labels: deploymentbuilder.MergeLabels(ctx.Component.GetLabels(), map[string]string{
			"engine":  Engine,
			"role":    Role,
			"managed": "true",
		}),
		Tags: ctx.Component.GetTags(),
	}, nil
}

func (r DeploymentRenderer) RenderPreview(ctx deploymentbuilder.PreviewContext) ([]*deploymentpb.RenderArtifact, error) {
	artifacts := []*deploymentpb.RenderArtifact{
		deploymentbuilder.CommandArtifact(ctx, Engine,
			deploymentbuilder.ArtifactID(ctx.Component.GetId(), "managed/marker"),
			"echo 'managed YDB is provider-managed; nothing to install on this node'",
			"managed YDB is provisioned by the cloud provider",
			map[string]string{"artifact": "managed"}),
		deploymentbuilder.RuntimeArtifact(ctx, Engine, "runtime/ydb-endpoint",
			"machine."+ctx.Node.GetId()+".endpoint.private.address"),
	}
	return artifacts, nil
}
