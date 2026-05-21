package agentqueue

import (
	"context"
	"strings"
	"testing"

	"github.com/gopherex/xlog"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/planner"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
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
		Database: &domain.Database{Kind: domain.Database_KIND_POSTGRES, Version: "16"},
		Topology: &domain.Topology{Machines: []*domain.Topology_Machine{{
			Id: "db-1", Cores: 4, MemoryGb: 16, DiskGb: 100,
			Components: []*domain.Topology_Component{{Id: "pg", Kind: domain.Topology_Component_KIND_DATABASE}},
		}}},
	}
	dag, err := planner.New().Compile(preset, nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// Simulate terraform_apply completed with vm ips.
	out, err := anypb.New(&ops.TfOperation_Output{OutputsJson: map[string][]byte{
		"vm_ips": []byte(`{"db-1":{"internal_ip":"10.0.0.5","nat_ip":"203.0.113.5"}}`),
	}})
	if err != nil {
		t.Fatalf("wrap output: %v", err)
	}
	findByExecutionID(dag, "terraform_apply").GetTaskState().Output = out

	// Park the agent command (the executor would have set it RUNNING).
	stroppy := findByExecutionID(dag, "install_and_run.run_stroppy")
	if stroppy == nil {
		t.Fatal("run_stroppy node not found")
	}
	stroppy.Status = primitive.Status_STATUS_RUNNING

	q := New(xlog.Default(), &fakeStore{dags: []*primitive.Dag{dag}})
	lease, err := q.Lease(context.Background(), "t1", &rtagent.Target{MachineId: "m1"})
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
