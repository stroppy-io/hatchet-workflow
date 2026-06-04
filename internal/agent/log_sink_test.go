package agent

import (
	"testing"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

func TestLogLinesFromChunkUsesDeploymentExecutionContext(t *testing.T) {
	ctx := commandLogContextFromEnv(map[string]string{
		deploymentbuilder.EnvRunID:                 "run-1",
		deploymentbuilder.EnvNodeExecutionID:       "deploy-step-1",
		deploymentbuilder.EnvParentNodeExecutionID: "component/postgres-master",
		deploymentbuilder.EnvPhase:                 "execute_deployment_plan",
		deploymentbuilder.EnvStageName:             "call_cmd: install postgres",
		deploymentbuilder.EnvComponentID:           "postgres-master",
		deploymentbuilder.EnvNodeID:                "node-1",
		deploymentbuilder.EnvStepID:                "120_install",
		deploymentbuilder.EnvAction:                "call_cmd",
		deploymentbuilder.EnvOperationMentions:     "postgres,pgdg",
	})

	lines := logLinesFromChunk(ctx, monitor.Stream_STREAM_STDERR, []byte("one\ntwo\n"))
	if got, want := len(lines), 2; got != want {
		t.Fatalf("lines = %d, want %d", got, want)
	}
	for i, line := range lines {
		if line.GetObservedAt() == nil {
			t.Fatalf("line %d observed_at is nil", i)
		}
		if got, want := line.GetRunId(), "run-1"; got != want {
			t.Fatalf("line %d run_id = %q, want %q", i, got, want)
		}
		if got, want := line.GetNodeExecutionId(), "deploy-step-1"; got != want {
			t.Fatalf("line %d node_execution_id = %q, want %q", i, got, want)
		}
		if got, want := line.GetParentNodeExecutionId(), "component/postgres-master"; got != want {
			t.Fatalf("line %d parent_node_execution_id = %q, want %q", i, got, want)
		}
		if got, want := line.GetPhase(), "execute_deployment_plan"; got != want {
			t.Fatalf("line %d phase = %q, want %q", i, got, want)
		}
		if got, want := line.GetStageName(), "call_cmd: install postgres"; got != want {
			t.Fatalf("line %d stage_name = %q, want %q", i, got, want)
		}
		if got, want := line.GetComponentId(), "postgres-master"; got != want {
			t.Fatalf("line %d component_id = %q, want %q", i, got, want)
		}
		if got, want := line.GetMachineId(), "node-1"; got != want {
			t.Fatalf("line %d machine_id = %q, want %q", i, got, want)
		}
		if got, want := line.GetUnit(), "call_cmd"; got != want {
			t.Fatalf("line %d unit = %q, want %q", i, got, want)
		}
		if got, want := line.GetStepId(), "120_install"; got != want {
			t.Fatalf("line %d step_id = %q, want %q", i, got, want)
		}
		if got, want := line.GetAction(), "call_cmd"; got != want {
			t.Fatalf("line %d action = %q, want %q", i, got, want)
		}
		if got, want := line.GetMentions(), []string{"postgres", "pgdg"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("line %d mentions = %v, want %v", i, got, want)
		}
		if got, want := line.GetStream(), monitor.Stream_STREAM_STDERR; got != want {
			t.Fatalf("line %d stream = %s, want %s", i, got, want)
		}
	}
	if got, want := lines[0].GetLine(), "one"; got != want {
		t.Fatalf("first line = %q, want %q", got, want)
	}
	if got, want := lines[1].GetLine(), "two"; got != want {
		t.Fatalf("second line = %q, want %q", got, want)
	}
}

func TestLogLinesFromChunkDropsLinesWithoutRunID(t *testing.T) {
	lines := logLinesFromChunk(commandLogContext{}, monitor.Stream_STREAM_STDOUT, []byte("orphan\n"))
	if len(lines) != 0 {
		t.Fatalf("lines without run id were emitted: %v", lines)
	}
}
