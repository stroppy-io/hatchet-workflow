package deployment

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

const (
	maxPipelineOutputs           = 256
	maxOperationCommandTextBytes = 2048
	maxOperationPreviewBytes     = 2048
	maxOperationArgvItems        = 32
	maxOperationArgvElementBytes = 256
)

// AgentStepStageName returns the compact runtime label for an executable agent
// step, derived from the action payload rather than from backend-specific ids.
func AgentStepStageName(step *deploymentpb.AgentStep) string {
	operation := AgentStepOperation(step)
	if operation != nil && operation.GetSummary() != "" {
		return shortNodeName("", operation.GetSummary())
	}
	action := AgentStepActionKind(step)
	if action == "" && step != nil {
		return step.GetId()
	}
	return action
}

// AgentStepOperation exposes an AgentStep action as a generic inspectable
// operation. It intentionally keys off the action oneof, never off step ids.
func AgentStepOperation(step *deploymentpb.AgentStep) *monitor.PipelineOperation {
	if step == nil {
		return nil
	}
	operation := &monitor.PipelineOperation{
		StepId:    step.GetId(),
		StepOrder: step.GetOrder(),
		Labels:    cloneStringMap(step.GetLabels()),
	}
	var corpus []string
	switch typed := step.GetAction().(type) {
	case *deploymentpb.AgentStep_CreateDir:
		dir := typed.CreateDir
		operation.Kind = monitor.OperationKind_OPERATION_KIND_CREATE_DIR
		operation.Target = dir.GetInfo().GetPath()
		operation.Summary = "create_dir: " + operation.Target
		corpus = append(corpus, operation.Target)
	case *deploymentpb.AgentStep_WriteFile:
		file := typed.WriteFile
		operation.Kind = monitor.OperationKind_OPERATION_KIND_WRITE_FILE
		operation.FilePath = file.GetInfo().GetPath()
		operation.Target = operation.FilePath
		operation.ContentPreview, operation.FileSizeBytes = fileContentPreview(file)
		operation.Summary = "write_file: " + operation.FilePath
		corpus = append(corpus, operation.FilePath, operation.ContentPreview)
	case *deploymentpb.AgentStep_FetchFile:
		file := typed.FetchFile
		operation.Kind = monitor.OperationKind_OPERATION_KIND_FETCH_FILE
		operation.FilePath = file.GetInfo().GetPath()
		operation.Target = operation.FilePath
		operation.ContentPreview, operation.FileSizeBytes = fileContentPreview(file)
		operation.Summary = "fetch_file: " + operation.FilePath
		corpus = append(corpus, operation.FilePath, operation.ContentPreview, fileRefURI(file))
	case *deploymentpb.AgentStep_CallCmd:
		cmd := typed.CallCmd
		operation.Kind = monitor.OperationKind_OPERATION_KIND_CALL_CMD
		commandText, argv := commandTextAndArgv(cmd)
		operation.CommandText = boundedString(commandText, maxOperationCommandTextBytes)
		operation.Argv = boundedStringSlice(argv, maxOperationArgvItems, maxOperationArgvElementBytes)
		operation.Target = commandTarget(cmd, commandText)
		corpus = append(corpus, operation.Target, operation.CommandText, strings.Join(operation.Argv, " "))
		if result := cmd.GetResult(); result != nil {
			operation.ResultAvailable = true
			operation.ExitCode = result.GetExitCode()
			operation.TimedOut = result.GetTimedOut()
			operation.Elapsed = result.GetElapsed()
			operation.StdoutPreview = boundedString(string(result.GetStdout()), maxOperationPreviewBytes)
			operation.StderrPreview = boundedString(string(result.GetStderr()), maxOperationPreviewBytes)
			operation.ResultSummary = commandResultSummary(result)
			corpus = append(corpus, operation.StdoutPreview, operation.StderrPreview)
		}
	default:
		operation.Kind = monitor.OperationKind_OPERATION_KIND_UNSPECIFIED
		operation.Target = step.GetId()
		operation.Summary = step.GetId()
		corpus = append(corpus, step.GetId())
	}
	operation.Mentions = extractMentions(corpus...)
	if operation.GetKind() == monitor.OperationKind_OPERATION_KIND_CALL_CMD {
		operation.Summary = commandOperationSummary(operation.Target, operation.Mentions)
	}
	if operation.GetSummary() == "" {
		operation.Summary = operation.GetTarget()
	}
	return operation
}

// DeploymentPlanOutputs returns structured render outputs for the deployment
// plan produced by RenderDeploymentPlanWorkflow. These outputs describe the
// rendered artifacts without requiring the UI to reverse-engineer AgentStep ids.
func DeploymentPlanOutputs(plan *deploymentpb.DeploymentPlan) []*monitor.PipelineOutput {
	if plan == nil {
		return nil
	}
	var components, steps, commands, files, dirs int
	for _, component := range plan.GetComponents() {
		if component == nil {
			continue
		}
		components++
		for _, step := range component.GetSteps() {
			if step == nil {
				continue
			}
			steps++
			switch step.GetAction().(type) {
			case *deploymentpb.AgentStep_CallCmd:
				commands++
			case *deploymentpb.AgentStep_WriteFile, *deploymentpb.AgentStep_FetchFile:
				files++
			case *deploymentpb.AgentStep_CreateDir:
				dirs++
			}
		}
	}
	outputs := []*monitor.PipelineOutput{
		{
			Kind:    monitor.OutputKind_OUTPUT_KIND_DEPLOYMENT_PLAN,
			Id:      "deployment_plan",
			Name:    "deployment plan",
			Summary: fmt.Sprintf("rendered %d components, %d steps (%d commands, %d files, %d dirs)", components, steps, commands, files, dirs),
			Count:   uint32(steps),
			Labels:  cloneStringMap(plan.GetLabels()),
		},
	}
	for _, component := range plan.GetComponents() {
		if component == nil || len(outputs) >= maxPipelineOutputs {
			continue
		}
		outputs = append(outputs, &monitor.PipelineOutput{
			Kind:        monitor.OutputKind_OUTPUT_KIND_SUMMARY,
			Id:          "component/" + component.GetComponentId(),
			Name:        component.GetComponentId(),
			Summary:     fmt.Sprintf("rendered %d steps on %s", len(component.GetSteps()), component.GetNodeId()),
			ComponentId: component.GetComponentId(),
			MachineId:   component.GetNodeId(),
			Count:       uint32(len(component.GetSteps())),
			Labels:      cloneStringMap(component.GetLabels()),
		})
		for _, step := range component.GetSteps() {
			if len(outputs) >= maxPipelineOutputs {
				break
			}
			if output := AgentStepRenderOutput(component, step); output != nil {
				outputs = append(outputs, output)
			}
		}
	}
	return outputs
}

// AgentStepRenderOutput describes one rendered AgentStep as a stage output.
func AgentStepRenderOutput(component *deploymentpb.ComponentDeployment, step *deploymentpb.AgentStep) *monitor.PipelineOutput {
	if component == nil || step == nil {
		return nil
	}
	operation := AgentStepOperation(step)
	if operation == nil {
		return nil
	}
	return &monitor.PipelineOutput{
		Kind:           monitor.OutputKind_OUTPUT_KIND_RENDER_ARTIFACT,
		Id:             "component/" + component.GetComponentId() + "/step/" + step.GetId(),
		Name:           step.GetId(),
		Summary:        operation.GetSummary(),
		ComponentId:    component.GetComponentId(),
		MachineId:      component.GetNodeId(),
		StepId:         step.GetId(),
		Action:         AgentStepActionKind(step),
		Target:         operation.GetTarget(),
		CommandText:    operation.GetCommandText(),
		ContentPreview: operation.GetContentPreview(),
		SizeBytes:      operation.GetFileSizeBytes(),
		Labels:         cloneStringMap(operation.GetLabels()),
	}
}

// AgentStepResultOutputs returns command result outputs for an executed step.
func AgentStepResultOutputs(component *deploymentpb.ComponentDeployment, step *deploymentpb.AgentStep) []*monitor.PipelineOutput {
	if component == nil || step == nil {
		return nil
	}
	operation := AgentStepOperation(step)
	if operation == nil || !operation.GetResultAvailable() {
		return nil
	}
	preview := operation.GetStdoutPreview()
	if stderr := operation.GetStderrPreview(); stderr != "" {
		if preview != "" {
			preview += "\n"
		}
		preview += "stderr:\n" + stderr
	}
	return []*monitor.PipelineOutput{
		{
			Kind:           monitor.OutputKind_OUTPUT_KIND_COMMAND_RESULT,
			Id:             "component/" + component.GetComponentId() + "/step/" + step.GetId() + "/result",
			Name:           "result",
			Summary:        operation.GetResultSummary(),
			ComponentId:    component.GetComponentId(),
			MachineId:      component.GetNodeId(),
			StepId:         step.GetId(),
			Action:         AgentStepActionKind(step),
			Target:         operation.GetTarget(),
			ContentPreview: preview,
			SizeBytes:      uint64(len(operation.GetStdoutPreview()) + len(operation.GetStderrPreview())),
			Labels:         cloneStringMap(operation.GetLabels()),
		},
	}
}

func commandTextAndArgv(cmd *common.Cmd) (string, []string) {
	if cmd == nil || cmd.GetSpec() == nil {
		return "", nil
	}
	switch typed := cmd.GetSpec().GetCommand().(type) {
	case *common.Cmd_Spec_Argv:
		args := append([]string(nil), typed.Argv.GetArgs()...)
		return strings.Join(args, " "), args
	case *common.Cmd_Spec_Script:
		return typed.Script.GetText(), nil
	default:
		return "", nil
	}
}

func commandTarget(cmd *common.Cmd, commandText string) string {
	if cmd == nil || cmd.GetSpec() == nil {
		return firstMeaningfulLine(commandText)
	}
	switch typed := cmd.GetSpec().GetCommand().(type) {
	case *common.Cmd_Spec_Argv:
		if len(typed.Argv.GetArgs()) > 0 {
			return typed.Argv.GetArgs()[0]
		}
	case *common.Cmd_Spec_Script:
		return firstMeaningfulLine(typed.Script.GetText())
	}
	return firstMeaningfulLine(commandText)
}

func commandOperationSummary(target string, mentions []string) string {
	summary := "call_cmd"
	if target != "" {
		summary += ": " + target
	}
	if len(mentions) == 0 {
		return summary
	}
	limit := len(mentions)
	if limit > 8 {
		limit = 8
	}
	return summary + " [mentions: " + strings.Join(mentions[:limit], ", ") + "]"
}

func commandResultSummary(result *common.Cmd_Result) string {
	if result == nil {
		return ""
	}
	parts := []string{"exit " + strconv.Itoa(int(result.GetExitCode()))}
	if result.GetTimedOut() {
		parts = append(parts, "timed out")
	}
	if elapsed := result.GetElapsed(); elapsed != nil {
		parts = append(parts, "elapsed "+elapsed.AsDuration().String())
	}
	if len(result.GetStdout()) > 0 {
		parts = append(parts, "stdout "+strconv.Itoa(len(result.GetStdout()))+"B")
	}
	if len(result.GetStderr()) > 0 {
		parts = append(parts, "stderr "+strconv.Itoa(len(result.GetStderr()))+"B")
	}
	return strings.Join(parts, ", ")
}

func firstMeaningfulLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || line == "set -e" || line == "set -eu" || line == "set -eux" {
			continue
		}
		return line
	}
	return strings.TrimSpace(text)
}

func fileContentPreview(file *common.File) (string, uint64) {
	if file == nil {
		return "", 0
	}
	switch typed := file.GetContent().(type) {
	case *common.File_Text:
		text := typed.Text
		return boundedString(text, maxOperationPreviewBytes), uint64(len(text))
	case *common.File_Bytes:
		return boundedString(string(typed.Bytes), maxOperationPreviewBytes), uint64(len(typed.Bytes))
	default:
		if ref := file.GetAsRef(); ref != nil {
			return ref.GetUri(), 0
		}
		return "", 0
	}
}

func boundedString(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}

func boundedStringSlice(in []string, maxItems, maxElementBytes int) []string {
	if len(in) == 0 || maxItems <= 0 {
		return nil
	}
	if len(in) > maxItems {
		in = in[:maxItems]
	}
	out := make([]string, 0, len(in))
	for _, value := range in {
		out = append(out, boundedString(value, maxElementBytes))
	}
	return out
}

func fileRefURI(file *common.File) string {
	if file == nil {
		return ""
	}
	ref := file.GetAsRef()
	if ref == nil {
		return ""
	}
	return ref.GetUri()
}

var mentionTokenRE = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_.-]{1,63}`)

func extractMentions(parts ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 16)
	for _, part := range parts {
		for _, raw := range mentionTokenRE.FindAllString(part, -1) {
			token := strings.ToLower(strings.Trim(raw, ".-_"))
			if len(token) < 3 || isShellNoiseToken(token) {
				continue
			}
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			out = append(out, token)
			if len(out) == 64 {
				return out
			}
		}
	}
	return out
}

func isShellNoiseToken(token string) bool {
	switch token {
	case "then", "else", "elif", "fi", "for", "while", "do", "done", "case", "esac", "set":
		return true
	default:
		return false
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func shortNodeName(action, detail string) string {
	const maxDetail = 96
	if detail == "" {
		return action
	}
	if len(detail) > maxDetail {
		detail = detail[:maxDetail-1] + "..."
	}
	if action == "" {
		return detail
	}
	return action + ": " + detail
}
