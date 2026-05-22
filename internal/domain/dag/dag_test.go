package dag

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
	"github.com/stroppy-io/stroppy-cloud/internal/runtime"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ── fakes for every Deps seam (proto-only, no real I/O) ──────────────────────

type fakeNet struct{}

func (fakeNet) AllocNetwork(*domain.Topology) *system.Network { return &system.Network{} }

type fakeQuota struct{}

func (fakeQuota) Request(*domain.Topology, *deployment.DeploymentIntent) ([]*deployment.QuotaRequest, error) {
	return nil, nil
}

type fakeDocker struct{ applied, destroyed bool }

func (f *fakeDocker) Apply(_ context.Context, d *deployment.Docker) (*deployment.Docker, error) {
	f.applied = true
	d.Output = &deployment.Docker_Output{Containers: map[string]*deployment.Docker_ContainerOutput{
		"m1": {InternalIp: "10.0.0.1"},
	}}
	return d, nil
}
func (f *fakeDocker) Destroy(context.Context, *deployment.Docker) error {
	f.destroyed = true
	return nil
}

type fakeTF struct{ applied, destroyed bool }

func (f *fakeTF) Apply(_ context.Context, y *deployment.Yandex) (*deployment.Yandex, error) {
	f.applied = true
	return y, nil
}
func (f *fakeTF) Destroy(context.Context, *deployment.Yandex) error { f.destroyed = true; return nil }

// fakeInstall returns a trivial server sub-dag (one no-op node) registered into reg.
type fakeInstall struct{ reg runtime.TasksRegistry }

func (f fakeInstall) Build(*domain.TestPreset, string) *primitive.Dag {
	f.reg.Register(runtime.NewTask[*emptypb.Empty, *emptypb.Empty]("install_noop",
		func(runtime.DagContext, *emptypb.Empty) (*emptypb.Empty, error) { return &emptypb.Empty{}, nil }))
	return &primitive.Dag{
		Id: "install-sub",
		Nodes: []*primitive.Dag_Node{{
			Id: "install_noop",
			Variant: &primitive.Dag_Node_TaskState_{TaskState: &primitive.Dag_Node_TaskState{
				Input: _toAny(&emptypb.Empty{}), HandlerName: "install_noop",
				Locus: primitive.Dag_Node_TaskState_EXECUTION_LOCUS_SERVER,
			}},
			Scheduling: &primitive.Dag_Node_Scheduling{},
		}},
	}
}

func dockerPreset() *domain.TestPreset {
	return &domain.TestPreset{
		Deployment: &deployment.DeploymentIntent{Provider: deployment.Provider_PROVIDER_DOCKER},
		Topology:   &domain.Topology{Machines: []*domain.Topology_Machine{{Id: "m1"}}},
	}
}

func nodeStatus(dag *primitive.Dag, id string) primitive.Status {
	for _, n := range dag.GetNodes() {
		if n.GetId() == id {
			return n.GetStatus()
		}
	}
	return primitive.Status_STATUS_UNSPECIFIED
}

// TestBuildTestDagDockerBranch runs the whole blueprint dag through the REAL runtime
// executor with a DOCKER preset: the docker branch runs, the yandex branch is
// skipped (provider switch via conditional edges), and the dag completes.
func TestBuildTestDagDockerBranch(t *testing.T) {
	reg := runtime.NewTaskRegistry()
	docker := &fakeDocker{}
	tf := &fakeTF{}
	deps := Deps{Docker: docker, TF: tf, Install: fakeInstall{reg: reg}}

	dep := BuildDeployment(dockerPreset(), &system.Network{}, nil)
	dag := BuildTestDag(dockerPreset(), dep, deps)
	RegisterProvisionHandlers(reg, deps)
	PprintDag(dag, false)
	preds := ProviderPredicates()
	for k, v := range runtime.DefaultPredicates() {
		preds[k] = v
	}
	runErr := runtime.NewExecutor(reg, runtime.WithPredicates(preds)).Run(context.Background(), dag)
	t.Logf("run err: %v; dag failure: %s", runErr, dag.GetExecution().GetFailure().GetMessage())

	if dag.GetStatus() != primitive.Status_STATUS_COMPLETED {
		for _, n := range dag.GetNodes() {
			t.Logf("node %s = %s (failure: %s)", n.GetId(), n.GetStatus(), n.GetExecution().GetFailure().GetMessage())
		}
		t.Fatalf("dag status = %s, want completed", dag.GetStatus())
	}
	if got := nodeStatus(dag, deployDocker); got != primitive.Status_STATUS_COMPLETED {
		t.Errorf("deployDocker = %s, want completed", got)
	}
	if got := nodeStatus(dag, deployYc); got != primitive.Status_STATUS_SKIPPED {
		t.Errorf("deployYc = %s, want skipped (provider switch)", got)
	}
	if !docker.applied {
		t.Error("docker provisioner Apply not called")
	}
	if tf.applied {
		t.Error("terraform Apply called on a docker run")
	}
	if !docker.destroyed {
		t.Error("docker teardown not called")
	}
}
