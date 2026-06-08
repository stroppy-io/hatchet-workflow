package workflows

import (
	"strings"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func appendServerStageLog(ctx workflow.Context, runID, phase, parentNodeExecutionID string, stream monitor.Stream, line string, path ...string) {
	lc := terraformLogContext(runID, phase, parentNodeExecutionID, path...)
	if lc.GetRunId() == "" || line == "" {
		return
	}
	_ = appendRunLogs(ctx, &monitor.LogLine{
		ObservedAt:            timestamppb.New(workflow.Now(ctx)),
		RunId:                 lc.GetRunId(),
		NodeExecutionId:       lc.GetNodeExecutionId(),
		ParentNodeExecutionId: lc.GetParentNodeExecutionId(),
		Phase:                 lc.GetPhase(),
		StageName:             lc.GetStageName(),
		Action:                lc.GetAction(),
		Mentions:              lc.GetMentions(),
		Source:                monitor.Source_SOURCE_SERVER,
		Unit:                  lc.GetUnit(),
		Stream:                stream,
		Line:                  line,
	})
}

func terraformLogContext(runID, phase, parentNodeExecutionID string, path ...string) *deploymentpb.Terraform_Operation_LogContext {
	name := ""
	if len(path) > 0 {
		name = path[len(path)-1]
	}
	return &deploymentpb.Terraform_Operation_LogContext{
		RunId:                 runID,
		NodeExecutionId:       actionStageExecutionID(append([]string{phase}, path...)...),
		ParentNodeExecutionId: parentNodeExecutionID,
		Phase:                 phase,
		StageName:             name,
		Action:                name,
		Unit:                  name,
		Mentions:              serverStageMentions(path...),
	}
}

func stampTerraformLogContext(input *deploymentpb.Terraform_Input, runID, phase, parentNodeExecutionID string, path ...string) {
	if input == nil {
		return
	}
	if input.Operation == nil {
		input.Operation = &deploymentpb.Terraform_Operation{}
	}
	input.Operation.LogContext = terraformLogContext(runID, phase, parentNodeExecutionID, path...)
}

func stampDockerLogContext(input *deploymentpb.Docker_Input, runID, phase, parentNodeExecutionID string, path ...string) {
	if input == nil {
		return
	}
	lc := terraformLogContext(runID, phase, parentNodeExecutionID, path...)
	input.LogContext = &deploymentpb.Docker_Input_LogContext{
		RunId:                 lc.GetRunId(),
		NodeExecutionId:       lc.GetNodeExecutionId(),
		ParentNodeExecutionId: lc.GetParentNodeExecutionId(),
		Phase:                 lc.GetPhase(),
		StageName:             lc.GetStageName(),
		Action:                lc.GetAction(),
		Unit:                  lc.GetUnit(),
		Mentions:              append([]string(nil), lc.GetMentions()...),
	}
}

func serverStageMentions(path ...string) []string {
	seen := make(map[string]struct{}, len(path)+2)
	out := make([]string, 0, len(path)+2)
	for _, token := range path {
		for _, part := range strings.FieldsFunc(token, func(r rune) bool { return r == '_' || r == '-' || r == '/' }) {
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			out = append(out, part)
		}
		if _, ok := seen[token]; !ok && token != "" {
			seen[token] = struct{}{}
			out = append(out, token)
		}
	}
	return out
}
