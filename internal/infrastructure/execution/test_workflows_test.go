package execution

import (
	"context"
	"testing"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

func TestTestWorkflowRequestStampsMonitorLabelsForStandaloneLaunch(t *testing.T) {
	workflows := &TestWorkflows{
		bootstrap: fakeStandaloneBootstrap{serverAddr: "https://control.example"},
	}
	rec := &models.TestRunRecord{
		Entity: &common.Entity{Id: "run-1", TenantId: "tenant-1"},
		Spec: &domainpb.TestRun{
			Id: "run-1",
			TopologySpec: &topologypb.TopologySpec{
				Nodes: []*topologypb.Node{
					{Id: "node-1", ComponentIds: []string{"postgres-master"}},
				},
				Components: []*topologypb.Component{
					{
						Id:     "postgres-master",
						Kind:   topologypb.Component_KIND_DATABASE,
						Engine: "postgres",
						Role:   "master",
					},
				},
			},
			InfrastructurePlan: &deploymentpb.InfrastructurePlan{
				Provider: deploymentpb.Provider_PROVIDER_DOCKER,
				Machines: []*deploymentpb.MachinePlan{
					{NodeId: "node-1"},
				},
			},
		},
	}

	req, err := workflows.testWorkflowRequest(context.Background(), rec)
	if err != nil {
		t.Fatalf("build workflow request: %v", err)
	}
	labels := req.GetTestRun().GetTopologySpec().GetLabels()
	if got, want := labels[deploymentbuilder.LabelServerAddr], "https://control.example"; got != want {
		t.Fatalf("server label = %q, want %q", got, want)
	}
	if got, want := labels[deploymentbuilder.LabelRunID], "run-1"; got != want {
		t.Fatalf("run label = %q, want %q", got, want)
	}
	if got := rec.GetSpec().GetTopologySpec().GetLabels()[deploymentbuilder.LabelServerAddr]; got != "" {
		t.Fatalf("persisted record spec mutated with server addr %q", got)
	}
}

type fakeStandaloneBootstrap struct {
	serverAddr string
}

func (f fakeStandaloneBootstrap) AgentBootstrap(context.Context) (*workflowpb.AgentBootstrap, error) {
	return &workflowpb.AgentBootstrap{ServerAddr: f.serverAddr}, nil
}
