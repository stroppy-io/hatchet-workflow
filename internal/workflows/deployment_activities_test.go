package workflows

import (
	"context"
	"testing"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"google.golang.org/protobuf/proto"
)

func TestServerLogWriterStampsLogContextAndBuffersPartialLines(t *testing.T) {
	sink := &recordingRunLogWriter{}
	writer := newServerLogWriter(context.Background(), sink, serverLogContextFromTerraform(&deploymentpb.Terraform_Operation_LogContext{
		RunId:                 "run-1",
		NodeExecutionId:       "stage/apply",
		ParentNodeExecutionId: "stage/process",
		Phase:                 stageInfrastructure,
		StageName:             actionTerraformApply,
		Action:                actionTerraformApply,
		Unit:                  actionTerraformApply,
		Mentions:              []string{"terraform", "apply"},
	}), monitor.Stream_STREAM_STDERR)

	if _, err := writer.Write([]byte("line one\npartial")); err != nil {
		t.Fatalf("write first chunk: %v", err)
	}
	if _, err := writer.Write([]byte(" line two\r\n")); err != nil {
		t.Fatalf("write second chunk: %v", err)
	}
	writer.Flush()

	if got, want := len(sink.lines), 2; got != want {
		t.Fatalf("lines = %d, want %d", got, want)
	}
	first := sink.lines[0]
	if got, want := first.GetLine(), "line one"; got != want {
		t.Fatalf("first line = %q, want %q", got, want)
	}
	second := sink.lines[1]
	if got, want := second.GetLine(), "partial line two"; got != want {
		t.Fatalf("second line = %q, want %q", got, want)
	}
	if got, want := second.GetRunId(), "run-1"; got != want {
		t.Fatalf("run_id = %q, want %q", got, want)
	}
	if got, want := second.GetNodeExecutionId(), "stage/apply"; got != want {
		t.Fatalf("node_execution_id = %q, want %q", got, want)
	}
	if got, want := second.GetSource(), monitor.Source_SOURCE_SERVER; got != want {
		t.Fatalf("source = %s, want %s", got, want)
	}
	if got, want := second.GetStream(), monitor.Stream_STREAM_STDERR; got != want {
		t.Fatalf("stream = %s, want %s", got, want)
	}
	if got, want := second.GetMentions(), []string{"terraform", "apply"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("mentions = %v, want %v", got, want)
	}
}

type recordingRunLogWriter struct {
	lines []*monitor.LogLine
}

func (w *recordingRunLogWriter) Write(_ context.Context, lines []*monitor.LogLine) error {
	for _, line := range lines {
		w.lines = append(w.lines, proto.Clone(line).(*monitor.LogLine))
	}
	return nil
}
