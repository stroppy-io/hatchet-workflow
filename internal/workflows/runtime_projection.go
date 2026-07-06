package workflows

import (
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"google.golang.org/protobuf/proto"
)

// Runtime-projection compaction keeps GetRunState query payloads (and the
// periodic runtime persistence snapshot) small: large command/log payloads
// on stages get truncated rather than persisted/queried in full.
//
// Extracted (verbatim) from the old test.go domainTestWorkflow so
// runrecipe.go (RunRecipeWorkflow) can keep using the same compaction rules
// after the old stage-machine workflow was deleted.
const (
	maxRuntimeProjectionOutputs   = 80
	maxRuntimeProjectionTextBytes = 512
	maxRuntimeProjectionListItems = 16
)

func compactRunStateForRuntimeProjection(state *workflowpb.RunState) *workflowpb.RunState {
	if state == nil {
		return nil
	}
	compact := proto.Clone(state).(*workflowpb.RunState)
	for _, stage := range compact.GetStages() {
		compactStageForRuntimeProjection(stage)
	}
	return compact
}

func compactStageForRuntimeProjection(stage *workflowpb.Stage) {
	if stage == nil {
		return
	}
	stage.ErrorMessage = compactRuntimeProjectionText(stage.GetErrorMessage())
	if op := stage.GetOperation(); op != nil {
		op.Command = nil
		op.File = nil
		op.Dir = nil
		op.Summary = compactRuntimeProjectionText(op.GetSummary())
		op.CommandText = compactRuntimeProjectionText(op.GetCommandText())
		op.Argv = compactRuntimeProjectionList(op.GetArgv())
		op.ContentPreview = compactRuntimeProjectionText(op.GetContentPreview())
		op.StdoutPreview = compactRuntimeProjectionText(op.GetStdoutPreview())
		op.StderrPreview = compactRuntimeProjectionText(op.GetStderrPreview())
		op.Mentions = compactRuntimeProjectionList(op.GetMentions())
	}
	outputs := stage.GetOutputs()
	if len(outputs) > maxRuntimeProjectionOutputs {
		outputs = outputs[:maxRuntimeProjectionOutputs]
		stage.Outputs = outputs
	}
	for _, output := range outputs {
		if output == nil {
			continue
		}
		output.Summary = compactRuntimeProjectionText(output.GetSummary())
		output.Target = compactRuntimeProjectionText(output.GetTarget())
		output.CommandText = compactRuntimeProjectionText(output.GetCommandText())
		output.ContentPreview = compactRuntimeProjectionText(output.GetContentPreview())
	}
}

func compactRuntimeProjectionText(value string) string {
	if len(value) <= maxRuntimeProjectionTextBytes {
		return value
	}
	return value[:maxRuntimeProjectionTextBytes]
}

func compactRuntimeProjectionList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	if len(values) > maxRuntimeProjectionListItems {
		values = values[:maxRuntimeProjectionListItems]
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, compactRuntimeProjectionText(value))
	}
	return out
}
