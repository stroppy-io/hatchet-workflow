package agentqueue

import (
	"context"
	"strings"
	"testing"

	"github.com/gopherex/xlog"
	"google.golang.org/protobuf/types/known/anypb"

	dagdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
)

type fakeStore struct{ dags []*primitive.Dag }

func (f *fakeStore) ListByTenant(_ context.Context, _ string, _ []primitive.Status) ([]*primitive.Dag, error) {
	return f.dags, nil
}
func (f *fakeStore) GetDag(_ context.Context, id string) (*primitive.Dag, error) {
	for _, d := range f.dags {
		if d.GetId() == id {
			return d, nil
		}
	}
	return nil, nil
}
func (f *fakeStore) SaveDag(_ context.Context, _ *primitive.Dag) error { return nil }

// TestLeaseResolvesBindings is the H27 keystone loop: terraform output ->
// render.Binding -> resolved agent command.
func TestLeaseResolvesBindings(t *testing.T) {
	preset := &domain.TestPreset{
		Database:   &domain.Database{Kind: domain.Database_KIND_POSTGRES, Version: "16"},
		Deployment: &deployment.DeploymentIntent{Provider: deployment.Provider_PROVIDER_DOCKER},
		Topology: &domain.Topology{Machines: []*domain.Topology_Machine{{
			Id: "db-1", Cores: 4, MemoryGb: 16, DiskGb: 100,
			Components: []*domain.Topology_Component{
				{Id: "pg", Kind: domain.Topology_Component_KIND_DATABASE},
				{Id: "load", Kind: domain.Topology_Component_KIND_STROPPY},
			},
		}}},
	}
	dep := dagdomain.BuildDeployment(preset, &system.Network{}, nil)
	dag := dagdomain.BuildTestDag(preset, dep, dagdomain.Deps{Install: dagdomain.RecipeInstallBuilder{}})

	// Simulate deployDocker completed: the deployment Output carries the container IP.
	out, err := anypb.New(&deployment.Deployment{
		Provider: deployment.Provider_PROVIDER_DOCKER,
		Deployment: &deployment.Deployment_Docker{Docker: &deployment.Docker{
			Output: &deployment.Docker_Output{Containers: map[string]*deployment.Docker_ContainerOutput{
				"db-1": {InternalIp: "10.0.0.5"},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("wrap output: %v", err)
	}
	for _, n := range dag.GetNodes() {
		if n.GetId() == "deployDocker" {
			n.GetTaskState().Output = out
		}
	}

	// Park the agent command (the executor would have set it RUNNING).
	stroppy := findByExecutionID(dag, "install_and_run.load.run_stroppy")
	if stroppy == nil {
		t.Fatal("run_stroppy node not found")
	}
	stroppy.Status = primitive.Status_STATUS_RUNNING

	q := New(xlog.Default(), &fakeStore{dags: []*primitive.Dag{dag}})
	// The load component is placed on machine db-1 (topology), so only an agent
	// reporting that machine may lease its command — machine routing.
	if lease, err := q.Lease(context.Background(), "t1", &rtagent.Target{MachineId: "other"}); err != nil {
		t.Fatalf("lease (wrong machine): %v", err)
	} else if lease != nil {
		t.Fatal("expected no lease for a machine that owns no ready node")
	}
	lease, err := q.Lease(context.Background(), "t1", &rtagent.Target{MachineId: "db-1"})
	if err != nil {
		t.Fatalf("lease: %v", err)
	}
	if lease == nil {
		t.Fatal("expected a lease")
	}
	args := strings.Join(lease.GetCommand().GetOperation().GetRunCmd().GetArgv().GetArgs(), " ")
	if strings.Contains(args, "__STROPPY_DB_HOST__") {
		t.Errorf("token not resolved: %q", args)
	}
	if !strings.Contains(args, "10.0.0.5") {
		t.Errorf("db ip not substituted: %q", args)
	}
}
