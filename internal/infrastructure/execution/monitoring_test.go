package execution

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBuildLogsQueryIncludesStructuredFilters(t *testing.T) {
	query := buildLogsQuery("run-1", &api.LogFilter{
		NodeExecutionIds:       []string{"deploy-step-1", "deploy-step-2"},
		ComponentIds:           []string{"postgres-master"},
		NodeIds:                []string{"node-1"},
		Phases:                 []string{"execute_deployment_plan"},
		ParentNodeExecutionIds: []string{"component/postgres-master"},
		StageNames:             []string{"call_cmd: install postgres"},
		StepIds:                []string{"120_install"},
		Actions:                []string{"call_cmd"},
		Mentions:               []string{"postgres", "pgdg"},
		Sources:                []monitor.Source{monitor.Source_SOURCE_COMMAND, monitor.Source_SOURCE_UNSPECIFIED, monitor.Source_SOURCE_COMMAND},
		Streams:                []monitor.Stream{monitor.Stream_STREAM_STDERR},
		Unit:                   "call_cmd",
		Search:                 `pg_ctl "start"`,
		Query:                  `level:error`,
	}, api.LogScrollDirection_LOG_SCROLL_DIRECTION_OLDER)

	for _, want := range []string{
		`run_id:"run-1"`,
		`(node_execution_id:"deploy-step-1" OR node_execution_id:"deploy-step-2")`,
		`component_id:"postgres-master"`,
		`machine_id:"node-1"`,
		`phase:"execute_deployment_plan"`,
		`parent_node_execution_id:"component/postgres-master"`,
		`stage_name:"call_cmd: install postgres"`,
		`step_id:"120_install"`,
		`action:"call_cmd"`,
		`(mentions:"postgres" OR mentions:"pgdg")`,
		`source:"command"`,
		`stream:"stderr"`,
		`unit:"call_cmd"`,
		`_msg:"pg_ctl \"start\""`,
		`level:error`,
		`| sort by (_time) desc`,
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("query %q does not contain %q", query, want)
		}
	}
	if strings.Contains(query, "node_id:") {
		t.Fatalf("query filters by node_id instead of persisted machine_id: %q", query)
	}
}

func TestDecodeLogLinesKeepsCorrelationFieldsAndSkipsMalformedRows(t *testing.T) {
	body := strings.NewReader(strings.Join([]string{
		`{"_time":"2026-06-03T10:00:00Z","_msg":"started","run_id":"run-1","line_no":42,"node_execution_id":"deploy-step-1","parent_node_execution_id":"component/postgres-master","phase":"execute_deployment_plan","stage_name":"call_cmd: install postgres","component_id":"postgres-master","machine_id":"node-1","step_id":"120_install","action":"call_cmd","mentions":"postgres,pgdg","source":"command","unit":"call_cmd","stream":"stderr"}`,
		`{not-json}`,
		`{"_msg":"fallback run","component_id":"postgres-replica","source":"file","stream":"stdout"}`,
	}, "\n"))

	lines, err := decodeLogLines(body, "run-1")
	if err != nil {
		t.Fatalf("decode log lines: %v", err)
	}
	if got, want := len(lines), 2; got != want {
		t.Fatalf("decoded lines = %d, want %d", got, want)
	}

	first := lines[0]
	if got, want := first.GetRunId(), "run-1"; got != want {
		t.Fatalf("run_id = %q, want %q", got, want)
	}
	if got, want := first.GetLineNo(), uint64(42); got != want {
		t.Fatalf("line_no = %d, want %d", got, want)
	}
	if got, want := first.GetCursor().GetSeq(), uint64(42); got != want {
		t.Fatalf("cursor seq = %d, want %d", got, want)
	}
	if got, want := first.GetNodeExecutionId(), "deploy-step-1"; got != want {
		t.Fatalf("node_execution_id = %q, want %q", got, want)
	}
	if got, want := first.GetParentNodeExecutionId(), "component/postgres-master"; got != want {
		t.Fatalf("parent_node_execution_id = %q, want %q", got, want)
	}
	if got, want := first.GetPhase(), "execute_deployment_plan"; got != want {
		t.Fatalf("phase = %q, want %q", got, want)
	}
	if got, want := first.GetStageName(), "call_cmd: install postgres"; got != want {
		t.Fatalf("stage_name = %q, want %q", got, want)
	}
	if got, want := first.GetComponentId(), "postgres-master"; got != want {
		t.Fatalf("component_id = %q, want %q", got, want)
	}
	if got, want := first.GetMachineId(), "node-1"; got != want {
		t.Fatalf("machine_id = %q, want %q", got, want)
	}
	if got, want := first.GetStepId(), "120_install"; got != want {
		t.Fatalf("step_id = %q, want %q", got, want)
	}
	if got, want := first.GetAction(), "call_cmd"; got != want {
		t.Fatalf("action = %q, want %q", got, want)
	}
	if got, want := first.GetMentions(), []string{"postgres", "pgdg"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("mentions = %v, want %v", got, want)
	}
	if got, want := first.GetSource(), monitor.Source_SOURCE_COMMAND; got != want {
		t.Fatalf("source = %s, want %s", got, want)
	}
	if got, want := first.GetStream(), monitor.Stream_STREAM_STDERR; got != want {
		t.Fatalf("stream = %s, want %s", got, want)
	}

	second := lines[1]
	if got, want := second.GetRunId(), "run-1"; got != want {
		t.Fatalf("fallback run_id = %q, want %q", got, want)
	}
	if got, want := second.GetLineNo(), uint64(2); got != want {
		t.Fatalf("fallback line_no = %d, want %d", got, want)
	}
	if got, want := second.GetComponentId(), "postgres-replica"; got != want {
		t.Fatalf("fallback component_id = %q, want %q", got, want)
	}
	if got, want := second.GetSource(), monitor.Source_SOURCE_FILE; got != want {
		t.Fatalf("fallback source = %s, want %s", got, want)
	}
}

func TestNormalizeLogLinesDefaultsAndDropsUnusableRows(t *testing.T) {
	lines := normalizeLogLines([]*monitor.LogLine{
		nil,
		{RunId: "", Line: "missing run"},
		{RunId: "run-1", Line: ""},
		{RunId: "run-1", Line: "hello"},
	})
	if got, want := len(lines), 1; got != want {
		t.Fatalf("normalized lines = %d, want %d", got, want)
	}
	line := lines[0]
	if line.GetObservedAt() == nil {
		t.Fatal("observed_at was not defaulted")
	}
	if got, want := line.GetSource(), monitor.Source_SOURCE_COMMAND; got != want {
		t.Fatalf("source = %s, want %s", got, want)
	}
	if got, want := line.GetStream(), monitor.Stream_STREAM_STDOUT; got != want {
		t.Fatalf("stream = %s, want %s", got, want)
	}
}

func TestInitialStreamCursorDefaultsToNowUnlessBackfilling(t *testing.T) {
	cursor := initialStreamCursor(nil, nil)
	if cursor == nil || cursor.GetObservedAt() == nil {
		t.Fatal("empty stream cursor did not default to now")
	}

	from := &monitor.LogCursor{ObservedAt: timestamppb.Now(), Seq: 12}
	if got := initialStreamCursor(from, nil); got != from {
		t.Fatal("explicit stream cursor was not preserved")
	}

	filterStart := timestamppb.Now()
	if got := initialStreamCursor(nil, &api.LogFilter{Start: filterStart}); got != nil {
		t.Fatalf("stream cursor with filter.start = %v, want nil so Query backfills from filter.start", got)
	}
}
