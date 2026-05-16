package verbs

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// RunShell executes the RunShell action and returns a Report.
// CommandId and MachineId are left empty — Dispatcher fills them in.
func RunShell(ctx context.Context, action *agentpb.Action) *agentpb.Report {
	rs := action.GetRunShell()
	if rs == nil || len(rs.GetArgv()) == 0 {
		return failReport("RunShell: empty argv")
	}

	argv := rs.GetArgv()
	var cmd *exec.Cmd
	if rs.GetShell() {
		// Join args and pass to /bin/sh -c.
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", strings.Join(argv, " "))
	} else {
		cmd = exec.CommandContext(ctx, argv[0], argv[1:]...)
	}

	if rs.GetCwd() != "" {
		cmd.Dir = rs.GetCwd()
	}
	if envMap := rs.GetEnv(); len(envMap) > 0 {
		for k, v := range envMap {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	if rs.GetStdin() != "" {
		cmd.Stdin = strings.NewReader(rs.GetStdin())
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	runErr := cmd.Run()

	exitCode := int32(0)
	status := agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED
	errMsg := ""

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = int32(exitErr.ExitCode())
		} else {
			exitCode = -1
		}
		// Check expected_exits list.
		expected := rs.GetExpectedExits()
		if len(expected) == 0 {
			expected = []int32{0}
		}
		found := false
		for _, e := range expected {
			if e == exitCode {
				found = true
				break
			}
		}
		if !found {
			status = agentpb.ReportStatus_REPORT_STATUS_FAILED
			errMsg = runErr.Error()
		}
	}

	return &agentpb.Report{
		Status:     status,
		Error:      errMsg,
		Output:     buf.Bytes(),
		ExitCode:   exitCode,
		FinishedAt: timestamppb.Now(),
	}
}

func failReport(msg string) *agentpb.Report {
	return &agentpb.Report{
		Status:     agentpb.ReportStatus_REPORT_STATUS_FAILED,
		Error:      msg,
		FinishedAt: timestamppb.Now(),
	}
}
