package ydbmanaged

import (
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

// DeploymentRenderer renders the managed-YDB database component. Managed YDB is
// provisioned by the cloud provider (terraform managed_ydb block); there is no
// VM, no stroppy agent, and nothing to install/configure/start on the node. The
// component carries a "managed: true" label so the deploy executor skips it
// entirely (it never tries to reach an agent task queue that does not exist),
// while the node/endpoint stays resolvable for the workload connection. A single
// no-op marker step is kept only to satisfy ComponentDeployment validation
// (Steps requires >=1 item); it is never executed.
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
