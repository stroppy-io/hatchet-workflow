package deployment

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

func TestBuildPlanUsesRegisteredRenderer(t *testing.T) {
	spec := fakeSpec()
	plan, err := BuildPlan(spec, fakeInfrastructureState(spec), BuildOptions{
		Renderers: NewRegistry(fakeRenderer{}),
	})
	if err != nil {
		t.Fatalf("build deployment plan: %v", err)
	}

	if err := plan.Validate(); err != nil {
		t.Fatalf("plan is invalid: %v", err)
	}

	components := deploymentsByID(plan)
	if !hasDependency(components["proxy"], "database") {
		t.Fatal("proxy does not depend on database")
	}
	if got, want := components["database"].GetGlobalPriority(), uint32(10); got != want {
		t.Fatalf("database priority = %d, want %d", got, want)
	}
}

func TestBuildPlanRejectsMissingRenderer(t *testing.T) {
	spec := fakeSpec()
	_, err := BuildPlan(spec, fakeInfrastructureState(spec), BuildOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildPlanRejectsMissingInfrastructureStateNode(t *testing.T) {
	spec := fakeSpec()
	state := fakeInfrastructureState(spec)
	state.Machines = state.Machines[:1]

	_, err := BuildPlan(spec, state, BuildOptions{
		Renderers: NewRegistry(fakeRenderer{}),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildPreviewUsesRegisteredRenderer(t *testing.T) {
	spec := fakeSpec()
	preview, err := BuildPreview(spec, PreviewOptions{
		Renderers: NewRegistry(fakeRenderer{}),
		Labels:    map[string]string{"preview": "true"},
	})
	if err != nil {
		t.Fatalf("build render preview: %v", err)
	}

	if err := preview.Validate(); err != nil {
		t.Fatalf("preview is invalid: %v", err)
	}
	if got, want := len(preview.GetComponents()), 2; got != want {
		t.Fatalf("preview components = %d, want %d", got, want)
	}
	artifact := renderArtifactByID(preview, "database/run")
	if artifact == nil {
		t.Fatal("database run artifact is missing")
	}
	if got, want := artifact.GetMutability(), deploymentpb.RenderArtifact_MUTABILITY_READ_ONLY; got != want {
		t.Fatalf("artifact mutability = %s, want %s", got, want)
	}
}

type fakeRenderer struct{}

func (r fakeRenderer) Supports(component *topologypb.Component) bool {
	return component.GetEngine() == "fake"
}

func (r fakeRenderer) RenderComponent(ctx RenderContext) (*deploymentpb.ComponentDeployment, error) {
	priority := uint32(20)
	if ctx.Component.GetRole() == "database" {
		priority = 10
	}

	return &deploymentpb.ComponentDeployment{
		ComponentId:           ctx.Component.GetId(),
		NodeId:                ctx.Node.GetId(),
		GlobalPriority:        priority,
		NodePriority:          priority,
		DependsOnComponentIds: DependencyIDs(ctx, nil),
		Steps: []*deploymentpb.AgentStep{
			CallCmdStep("run", 1, "true"),
		},
		Status: common.Status_STATUS_PENDING,
		Labels: MergeLabels(ctx.Component.GetLabels(), map[string]string{"rendered": "true"}),
		Tags:   Tags("fake"),
	}, nil
}

func (r fakeRenderer) RenderPreview(ctx PreviewContext) ([]*deploymentpb.RenderArtifact, error) {
	return []*deploymentpb.RenderArtifact{
		{
			Id:           ArtifactID(ctx.Component.GetId(), "run"),
			ComponentId:  ctx.Component.GetId(),
			Kind:         deploymentpb.RenderArtifact_KIND_COMMAND,
			Origin:       deploymentpb.RenderArtifact_ORIGIN_SYSTEM,
			Mutability:   deploymentpb.RenderArtifact_MUTABILITY_READ_ONLY,
			LockReason:   "fake command",
			Artifact:     &deploymentpb.RenderArtifact_Cmd{Cmd: ShellCmd("true")},
			RendererName: "fake",
			Labels:       map[string]string{"rendered": "true"},
			Tags:         Tags("fake"),
		},
	}, nil
}

func fakeSpec() *topologypb.TopologySpec {
	port := uint32(5432)
	return &topologypb.TopologySpec{
		Nodes: []*topologypb.Node{
			{Id: "node-a", ComponentIds: []string{"database"}},
			{Id: "node-b", ComponentIds: []string{"proxy"}},
		},
		Components: []*topologypb.Component{
			{Id: "database", Kind: topologypb.Component_KIND_DATABASE, Engine: "fake", Role: "database"},
			{Id: "proxy", Kind: topologypb.Component_KIND_PROXY, Engine: "fake", Role: "proxy"},
		},
		Connections: []*topologypb.Connection{
			{
				FromComponentId: "proxy",
				ToComponentId:   "database",
				Kind:            topologypb.Connection_KIND_PROXY,
				Protocol:        topologypb.Connection_PROTOCOL_TCP,
				Mode:            topologypb.Connection_MODE_REQUEST,
				EndpointName:    "database",
				Port:            &port,
			},
		},
	}
}

func fakeInfrastructureState(spec *topologypb.TopologySpec) *deploymentpb.InfrastructureState {
	state := &deploymentpb.InfrastructureState{
		Provider: deploymentpb.Provider_PROVIDER_DOCKER,
		Machines: make([]*deploymentpb.MachineState, 0, len(spec.GetNodes())),
	}

	for _, node := range spec.GetNodes() {
		privatePort := uint32(22)
		state.Machines = append(state.Machines, &deploymentpb.MachineState{
			NodeId:             node.GetId(),
			ProviderResourceId: "container-" + node.GetId(),
			Status:             common.Status_STATUS_DEPLOYED,
			Endpoints: []*deploymentpb.Endpoint{
				{
					Name:    "private",
					Address: "10.0.0.1",
					Port:    &privatePort,
				},
			},
		})
	}

	return state
}

func deploymentsByID(plan *deploymentpb.DeploymentPlan) map[string]*deploymentpb.ComponentDeployment {
	components := make(map[string]*deploymentpb.ComponentDeployment, len(plan.GetComponents()))
	for _, component := range plan.GetComponents() {
		components[component.GetComponentId()] = component
	}
	return components
}

func hasDependency(component *deploymentpb.ComponentDeployment, dependency string) bool {
	for _, candidate := range component.GetDependsOnComponentIds() {
		if candidate == dependency {
			return true
		}
	}
	return false
}

func renderArtifactByID(preview *deploymentpb.RenderPreview, artifactID string) *deploymentpb.RenderArtifact {
	for _, artifact := range preview.GetArtifacts() {
		if artifact.GetId() == artifactID {
			return artifact
		}
	}
	return nil
}
