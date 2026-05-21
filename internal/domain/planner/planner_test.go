package planner

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/runtime"
)

func singlePreset() *domain.TestPreset {
	return &domain.TestPreset{
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				{Id: "db-1", Cores: 4, MemoryGb: 16, DiskGb: 100, DataDisksGb: []uint64{200}},
			},
		},
	}
}

func findNode(dag *primitive.Dag, id string) *primitive.Dag_Node {
	for _, n := range dag.GetNodes() {
		if n.GetId() == id {
			return n
		}
	}
	return nil
}

func TestCompileSingleProducesValidDag(t *testing.T) {
	dag, err := New().Compile(singlePreset(), nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := runtime.ValidateDag(dag); err != nil {
		t.Fatalf("validate dag: %v", err)
	}
	wantNodes := []string{"render_config", "terraform_apply", "install_and_run", "collect_results", "terraform_destroy"}
	for _, id := range wantNodes {
		if findNode(dag, id) == nil {
			t.Errorf("missing node %q", id)
		}
	}
	if !findNode(dag, "terraform_destroy").GetScheduling().GetAlwaysRun() {
		t.Error("terraform_destroy must be always_run")
	}
}

func TestTerraformApplyCarriesModuleAndVars(t *testing.T) {
	dag, err := New().Compile(singlePreset(), nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	apply := findNode(dag, "terraform_apply")
	input := apply.GetTaskState().GetInput()
	if input == nil {
		t.Fatal("terraform_apply has no input")
	}
	var op ops.TfOperation
	if err := input.UnmarshalTo(&op); err != nil {
		t.Fatalf("unmarshal TfOperation: %v", err)
	}
	if op.GetInput().GetAction() != ops.TfOperation_ACTION_APPLY {
		t.Errorf("action = %s, want APPLY", op.GetInput().GetAction())
	}
	if len(op.GetInput().GetFiles()) == 0 {
		t.Error("expected embedded .tf module files")
	}
	if op.GetInput().GetVarFile() == nil {
		t.Error("expected terraform.tfvars.json var_file")
	}

	// apply + destroy must share the workdir for crash-safe teardown.
	var destroyOp ops.TfOperation
	if err := findNode(dag, "terraform_destroy").GetTaskState().GetInput().UnmarshalTo(&destroyOp); err != nil {
		t.Fatalf("unmarshal destroy: %v", err)
	}
	if op.GetInput().GetWorkdirId() != destroyOp.GetInput().GetWorkdirId() {
		t.Error("apply and destroy must share workdir_id")
	}
}

func TestTfvarsFromParamsUseProtoNames(t *testing.T) {
	params := &DeploymentParams{
		Platform:    deployment.Yandex_PLATFORM_ID_STANDARD_V3,
		Zone:        deployment.Yandex_ZONE_RU_CENTRAL1_A,
		ImageID:     "img-1",
		NetworkID:   "net-1",
		NetworkName: "n",
		NetworkCIDR: "10.0.0.0/16",
	}
	dag, err := New().Compile(singlePreset(), params)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var op ops.TfOperation
	if err := findNode(dag, "terraform_apply").GetTaskState().GetInput().UnmarshalTo(&op); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	vars := op.GetInput().GetVarFile().GetContent().GetText()
	for _, want := range []string{"network_id", "net-1", "platform_id", "standard-v3", "boot_disk_gb"} {
		if !strings.Contains(vars, want) {
			t.Errorf("tfvars missing %q:\n%s", want, vars)
		}
	}
	if strings.Contains(vars, "networkId") {
		t.Errorf("tfvars used lowerCamel (want proto snake_case):\n%s", vars)
	}
}

func TestInstallSubDagUsesAgentCommands(t *testing.T) {
	dag, err := New().Compile(singlePreset(), nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	sub := findNode(dag, "install_and_run").GetSubDag()
	if sub == nil {
		t.Fatal("install_and_run is not a sub_dag")
	}
	if len(sub.GetNodes()) == 0 {
		t.Fatal("install sub_dag has no nodes")
	}
	for _, n := range sub.GetNodes() {
		ts := n.GetTaskState()
		if ts.GetHandlerName() != HandlerAgentCommand {
			t.Errorf("node %q handler = %q, want %q", n.GetId(), ts.GetHandlerName(), HandlerAgentCommand)
		}
		if ts.GetLocus() != primitive.Dag_Node_TaskState_EXECUTION_LOCUS_AGENT {
			t.Errorf("node %q must be agent-locus", n.GetId())
		}
	}
}
