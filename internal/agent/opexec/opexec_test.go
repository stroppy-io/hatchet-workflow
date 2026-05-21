package opexec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
)

func TestExecuteWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "conf")
	res, err := New().Execute(context.Background(), &ops.Operation{
		Kind: ops.Operation_KIND_WRITE_FILE,
		Operation: &ops.Operation_WriteFile{WriteFile: &system.File{
			Info: &system.File_Info{Path: path},
			Source: &system.File_Content_{Content: &system.File_Content{
				Content: &system.File_Content_Text{Text: "hello"},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if res.GetWriteFile() == nil {
		t.Fatal("expected write_file ack")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "hello" {
		t.Fatalf("file = %q err=%v", data, err)
	}
}

func TestExecuteRunCmd(t *testing.T) {
	res, err := New().Execute(context.Background(), &ops.Operation{
		Kind: ops.Operation_KIND_RUN_CMD,
		Operation: &ops.Operation_RunCmd{RunCmd: &system.Cmd_Spec{
			Command: &system.Cmd_Spec_Argv{Argv: &system.Cmd_Argv{Args: []string{"echo", "hi"}}},
		}},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	cr := res.GetRunCmd()
	if cr.GetExitCode() != 0 {
		t.Errorf("exit = %d", cr.GetExitCode())
	}
	if !strings.Contains(string(cr.GetStdout()), "hi") {
		t.Errorf("stdout = %q", cr.GetStdout())
	}
}

func TestExecuteRunCmdNonZeroExitIsResultNotError(t *testing.T) {
	res, err := New().Execute(context.Background(), &ops.Operation{
		Kind: ops.Operation_KIND_RUN_CMD,
		Operation: &ops.Operation_RunCmd{RunCmd: &system.Cmd_Spec{
			Command: &system.Cmd_Spec_Script{Script: &system.Cmd_Script{Text: "exit 3"}},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.GetRunCmd().GetExitCode() != 3 {
		t.Errorf("exit = %d, want 3", res.GetRunCmd().GetExitCode())
	}
}
