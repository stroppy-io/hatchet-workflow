package verbs

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Systemctl runs `systemctl <verb> [--now] <unit>`.
func Systemctl(ctx context.Context, action *agentpb.Action) *agentpb.Report {
	op := action.GetSystemctl()
	if op == nil {
		return failReport("Systemctl: nil payload")
	}

	verb := systemctlVerbStr(op.GetVerb())
	if verb == "" {
		return failReport(fmt.Sprintf("Systemctl: unknown verb %v", op.GetVerb()))
	}
	unit := op.GetUnit()
	if unit == "" {
		return failReport("Systemctl: empty unit")
	}

	args := []string{verb}
	if op.GetNow() {
		args = append(args, "--now")
	}
	args = append(args, unit)

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	exitCode := int32(0)
	status := agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED
	errMsg := ""

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = int32(exitErr.ExitCode())
		} else {
			exitCode = -1
		}
		status = agentpb.ReportStatus_REPORT_STATUS_FAILED
		errMsg = err.Error()
	}

	return &agentpb.Report{
		Status:     status,
		Error:      errMsg,
		Output:     buf.Bytes(),
		ExitCode:   exitCode,
		FinishedAt: timestamppb.Now(),
	}
}

func systemctlVerbStr(v agentpb.SystemctlOp_Verb) string {
	switch v {
	case agentpb.SystemctlOp_VERB_START:
		return "start"
	case agentpb.SystemctlOp_VERB_STOP:
		return "stop"
	case agentpb.SystemctlOp_VERB_RESTART:
		return "restart"
	case agentpb.SystemctlOp_VERB_RELOAD:
		return "reload"
	case agentpb.SystemctlOp_VERB_ENABLE:
		return "enable"
	case agentpb.SystemctlOp_VERB_DISABLE:
		return "disable"
	case agentpb.SystemctlOp_VERB_STATUS:
		return "status"
	default:
		return ""
	}
}
