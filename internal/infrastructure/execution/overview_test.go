package execution

import (
	"context"
	"errors"
	"testing"
	"time"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestOverviewGetFallsBackToPersistedTerminalRecord(t *testing.T) {
	started := timestamppb.New(time.Unix(10, 0))
	finished := timestamppb.New(time.Unix(70, 0))
	reader := &OverviewReader{
		tc: fakeRunStateQuerier{err: errors.New("workflow closed")},
		store: fakeSnapshotStore{
			run: &models.TestRunRecord{
				Entity:     &common.Entity{Id: "run-1"},
				Status:     common.Status_STATUS_COMPLETED,
				SuiteRunId: "suite-run-1",
				Summary: &models.TestRunRecord_Summary{
					StartedAt:   started,
					FinishedAt:  finished,
					Duration:    durationpb.New(time.Minute),
					ProgressPct: 100,
				},
			},
			suite: &models.SuiteRunRecord{
				Entity: &common.Entity{Id: "suite-run-1"},
				Status: common.Status_STATUS_COMPLETED,
			},
		},
	}

	snap, err := reader.Get(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}

	if got := snap.GetOverview().GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("overview status = %s, want %s", got, common.Status_STATUS_COMPLETED)
	}
	if got := snap.GetOverview().GetProgressPct(); got != 100 {
		t.Fatalf("overview progress = %d, want 100", got)
	}
	if got := snap.GetOverview().GetStartedAt(); got != started {
		t.Fatalf("overview started_at = %v, want persisted value", got)
	}
	if got := snap.GetRun().GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("run status = %s, want %s", got, common.Status_STATUS_COMPLETED)
	}
	if got := snap.GetSuiteRun().GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("suite run status = %s, want %s", got, common.Status_STATUS_COMPLETED)
	}
}

func TestOverviewMergesDeploymentPlanActionsIntoPipeline(t *testing.T) {
	rec := &models.TestRunRecord{
		Entity: &common.Entity{Id: "run-1"},
		DeploymentPlan: &deploymentpb.DeploymentPlan{
			Components: []*deploymentpb.ComponentDeployment{
				{
					ComponentId: "postgres-master",
					NodeId:      "node-1",
					Status:      common.Status_STATUS_DEPLOYMENT,
					Steps: []*deploymentpb.AgentStep{
						{
							Id:     "010_write_config",
							Order:  10,
							Status: common.Status_STATUS_DEPLOYED,
							Action: &deploymentpb.AgentStep_WriteFile{WriteFile: &common.File{
								Info:    &common.File_Info{Path: "/etc/postgresql/postgresql.conf"},
								Content: &common.File_Text{Text: "shared_buffers = 1GB\n"},
							}},
							Labels: map[string]string{
								deploymentbuilder.LabelNodeExecutionID: deploymentbuilder.StepExecutionID("postgres-master", "010_write_config"),
							},
						},
					},
					Labels: map[string]string{
						deploymentbuilder.LabelNodeExecutionID: deploymentbuilder.ComponentExecutionID("postgres-master"),
					},
				},
			},
		},
	}
	overview := projectOverview("run-1", &workflowpb.RunState{
		Status: common.Status_STATUS_RUNNING,
		Stages: []*workflowpb.Stage{
			{
				NodeExecutionId: deploymentbuilder.StageExecutionID(executeDeploymentPlanNodeName),
				Name:            executeDeploymentPlanNodeName,
				Status:          common.Status_STATUS_RUNNING,
				Attempt:         1,
			},
		},
	})
	overview = mergeOverviewWithRecord(overview, rec)

	root := overview.GetPipeline().GetRoots()[0]
	if got := len(root.GetChildren()); got != 1 {
		t.Fatalf("component children = %d, want 1", got)
	}
	component := root.GetChildren()[0]
	if got, want := component.GetNodeExecutionId(), deploymentbuilder.ComponentExecutionID("postgres-master"); got != want {
		t.Fatalf("component node_execution_id = %q, want %q", got, want)
	}
	if got := len(component.GetChildren()); got != 1 {
		t.Fatalf("step children = %d, want 1", got)
	}
	step := component.GetChildren()[0]
	if got, want := step.GetNodeExecutionId(), deploymentbuilder.StepExecutionID("postgres-master", "010_write_config"); got != want {
		t.Fatalf("step node_execution_id = %q, want %q", got, want)
	}
	if got, want := step.GetLogRef().GetRunId(), "run-1"; got != want {
		t.Fatalf("step log_ref run_id = %q, want %q", got, want)
	}
	if got, want := step.GetLogRef().GetNodeExecutionId(), deploymentbuilder.StepExecutionID("postgres-master", "010_write_config"); got != want {
		t.Fatalf("step log_ref node_execution_id = %q, want %q", got, want)
	}
	if got, want := step.GetLogRef().GetComponentId(), "postgres-master"; got != want {
		t.Fatalf("step log_ref component_id = %q, want %q", got, want)
	}
}

func TestOverviewProjectsWorkersFromPersistedTopology(t *testing.T) {
	rec := &models.TestRunRecord{
		Entity: &common.Entity{Id: "run-1"},
		InfrastructureState: &deploymentpb.InfrastructureState{
			Machines: []*deploymentpb.MachineState{
				{
					NodeId: "node-1",
					Status: common.Status_STATUS_DEPLOYED,
					Endpoints: []*deploymentpb.Endpoint{
						{Name: "public", Address: "203.0.113.10"},
						{Name: "private", Address: "10.0.0.10"},
					},
				},
			},
		},
		DeploymentPlan: &deploymentpb.DeploymentPlan{
			Components: []*deploymentpb.ComponentDeployment{
				{
					ComponentId: "postgres-master",
					NodeId:      "node-1",
					Status:      common.Status_STATUS_DEPLOYMENT,
					Steps: []*deploymentpb.AgentStep{
						{
							Id:     "020_start",
							Status: common.Status_STATUS_DEPLOYMENT,
							Action: &deploymentpb.AgentStep_CallCmd{CallCmd: &common.Cmd{}},
							Labels: map[string]string{
								deploymentbuilder.LabelNodeExecutionID: deploymentbuilder.StepExecutionID("postgres-master", "020_start"),
							},
						},
					},
				},
			},
		},
	}

	overview := mergeOverviewWithRecord(projectOverview("run-1", &workflowpb.RunState{
		Status: common.Status_STATUS_RUNNING,
		Stages: []*workflowpb.Stage{
			{
				NodeExecutionId: deploymentbuilder.StageExecutionID(executeDeploymentPlanNodeName),
				Name:            executeDeploymentPlanNodeName,
				Status:          common.Status_STATUS_RUNNING,
			},
		},
	}), rec)

	workers := overview.GetWorkers()
	if got, want := len(workers), 1; got != want {
		t.Fatalf("workers = %d, want %d", got, want)
	}
	worker := workers[0]
	if got, want := worker.GetId(), "agent/node-1"; got != want {
		t.Fatalf("worker id = %q, want %q", got, want)
	}
	if got, want := worker.GetMachineId(), "node-1"; got != want {
		t.Fatalf("worker machine_id = %q, want %q", got, want)
	}
	if got, want := worker.GetHost(), "10.0.0.10"; got != want {
		t.Fatalf("worker host = %q, want %q", got, want)
	}
	if !worker.GetOnline() {
		t.Fatal("worker online = false, want true for deployed machine")
	}
	if got, want := worker.GetStatus(), common.Status_STATUS_RUNNING; got != want {
		t.Fatalf("worker status = %s, want %s", got, want)
	}
	if got, want := worker.GetCurrentNodeExecutionId(), deploymentbuilder.StepExecutionID("postgres-master", "020_start"); got != want {
		t.Fatalf("worker current_node_execution_id = %q, want %q", got, want)
	}
}

func TestOverviewStreamClosesAfterPersistedTerminalSnapshot(t *testing.T) {
	reader := &OverviewReader{
		tc: fakeRunStateQuerier{err: errors.New("workflow closed")},
		store: fakeSnapshotStore{
			run: &models.TestRunRecord{
				Entity: &common.Entity{Id: "run-1"},
				Status: common.Status_STATUS_COMPLETED,
				Summary: &models.TestRunRecord_Summary{
					ProgressPct: 100,
				},
			},
		},
	}

	ch, err := reader.Stream(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("stream overview: %v", err)
	}

	select {
	case snap, ok := <-ch:
		if !ok {
			t.Fatal("stream closed before emitting snapshot")
		}
		if got := snap.GetOverview().GetStatus(); got != common.Status_STATUS_COMPLETED {
			t.Fatalf("overview status = %s, want %s", got, common.Status_STATUS_COMPLETED)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for stream snapshot")
	}

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("stream stayed open after terminal snapshot")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for stream close")
	}
}

type fakeRunStateQuerier struct {
	state *workflowpb.RunState
	err   error
}

func (f fakeRunStateQuerier) GetRunState(context.Context, string, string) (*workflowpb.RunState, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.state, nil
}

type fakeSnapshotStore struct {
	run   *models.TestRunRecord
	suite *models.SuiteRunRecord
}

func (f fakeSnapshotStore) RunRecord(context.Context, string) (*models.TestRunRecord, error) {
	return f.run, nil
}

func (f fakeSnapshotStore) SuiteRun(context.Context, string) (*models.SuiteRunRecord, error) {
	return f.suite, nil
}
