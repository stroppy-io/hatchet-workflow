// Package opexec is the agent's generic on-host operation executor: it runs the
// six primitive ops.Operation kinds (WRITE_FILE, APPEND_FILE, MAKE_DIR,
// MAKE_TEMP_DIR, READ_OS_INFO, RUN_CMD) and returns an ops.Operation_Result. The
// agent is domain-agnostic — per-engine recipes are planner data, decomposed into
// these ops (H29/H30). Recast of the generic parts of the old agent executor.
package opexec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
)

const (
	defaultFileMode = 0o644
	defaultDirMode  = 0o755
	defaultShell    = "/bin/sh"
)

// Executor runs operations on the local host.
type Executor struct{}

// New builds an Executor.
func New() *Executor { return &Executor{} }

// Execute runs one operation and returns its result. A non-zero command exit is
// reported inside the result (Cmd_Result.exit_code), not as a Go error; an error
// is returned only when the operation could not be performed at all.
func (e *Executor) Execute(ctx context.Context, op *ops.Operation) (*ops.Operation_Result, error) {
	switch v := op.GetOperation().(type) {
	case *ops.Operation_WriteFile:
		return writeFile(v.WriteFile, false)
	case *ops.Operation_AppendFile:
		return writeFile(v.AppendFile, true)
	case *ops.Operation_MakeDir:
		return makeDir(v.MakeDir)
	case *ops.Operation_MakeTempDir:
		return makeTempDir(v.MakeTempDir)
	case *ops.Operation_ReadOsInfo_:
		return readOSInfo()
	case *ops.Operation_RunCmd:
		return runCmd(ctx, v.RunCmd)
	default:
		return nil, fmt.Errorf("opexec: unsupported operation kind %s", op.GetKind())
	}
}

func writeFile(f *system.File, appendMode bool) (*ops.Operation_Result, error) {
	path := f.GetInfo().GetPath()
	if path == "" {
		return nil, fmt.Errorf("opexec: file path is empty")
	}
	mode := os.FileMode(defaultFileMode)
	if m := f.GetInfo().GetMode(); m != 0 {
		mode = os.FileMode(m)
	}
	if err := os.MkdirAll(filepath.Dir(path), defaultDirMode); err != nil {
		return nil, fmt.Errorf("opexec: mkdir for %s: %w", path, err)
	}
	if appendMode {
		fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, mode)
		if err != nil {
			return nil, fmt.Errorf("opexec: open %s: %w", path, err)
		}
		defer fh.Close()
		if _, err := fh.Write(fileContent(f)); err != nil {
			return nil, fmt.Errorf("opexec: append %s: %w", path, err)
		}
		return ack(true), nil
	}
	if err := os.WriteFile(path, fileContent(f), mode); err != nil {
		return nil, fmt.Errorf("opexec: write %s: %w", path, err)
	}
	return ack(false), nil
}

func makeDir(d *system.Dir) (*ops.Operation_Result, error) {
	path := d.GetInfo().GetPath()
	if path == "" {
		return nil, fmt.Errorf("opexec: dir path is empty")
	}
	mode := os.FileMode(defaultDirMode)
	if m := d.GetInfo().GetMode(); m != 0 {
		mode = os.FileMode(m)
	}
	if err := os.MkdirAll(path, mode); err != nil {
		return nil, fmt.Errorf("opexec: mkdir %s: %w", path, err)
	}
	return &ops.Operation_Result{Result: &ops.Operation_Result_MakeDir{MakeDir: &ops.Operation_Ack{}}}, nil
}

func makeTempDir(t *system.Dir_Temp) (*ops.Operation_Result, error) {
	dir, err := os.MkdirTemp(t.GetBase(), t.GetPattern())
	if err != nil {
		return nil, fmt.Errorf("opexec: make temp dir: %w", err)
	}
	return &ops.Operation_Result{Result: &ops.Operation_Result_MakeTempDir{
		MakeTempDir: &system.Dir{Info: &system.Dir_Info{Path: dir}},
	}}, nil
}

// readOSInfo returns host facts.
//
// TODO(opexec): only an empty OsInfo is returned — recast the old host probe
// (release/kernel/cpu/memory/filesystems/block devices). Reported.
func readOSInfo() (*ops.Operation_Result, error) {
	return &ops.Operation_Result{Result: &ops.Operation_Result_ReadOsInfo{ReadOsInfo: &system.OsInfo{}}}, nil
}

func runCmd(ctx context.Context, spec *system.Cmd_Spec) (*ops.Operation_Result, error) {
	if to := spec.GetTimeout().AsDuration(); to > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, to)
		defer cancel()
	}

	var cmd *exec.Cmd
	switch {
	case spec.GetArgv() != nil:
		args := spec.GetArgv().GetArgs()
		if len(args) == 0 {
			return nil, fmt.Errorf("opexec: empty argv")
		}
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	case spec.GetScript() != nil:
		shell := spec.GetScript().GetShell()
		if shell == "" {
			shell = defaultShell
		}
		cmd = exec.CommandContext(ctx, shell, "-c", spec.GetScript().GetText())
	default:
		return nil, fmt.Errorf("opexec: command has neither argv nor script")
	}
	cmd.Dir = spec.GetCwd()
	if stdin := spec.GetStdin(); len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	// TODO(opexec): apply spec.env once the agent's env model is wired.

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	result := &system.Cmd_Result{
		ExitCode: int32(cmd.ProcessState.ExitCode()),
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		TimedOut: ctx.Err() == context.DeadlineExceeded,
		Elapsed:  durationpb.New(elapsed),
	}
	// A start/exec failure with no process state is a real error.
	if err != nil && cmd.ProcessState == nil {
		return nil, fmt.Errorf("opexec: run command: %w", err)
	}
	return &ops.Operation_Result{Result: &ops.Operation_Result_RunCmd{RunCmd: result}}, nil
}

func ack(appendMode bool) *ops.Operation_Result {
	if appendMode {
		return &ops.Operation_Result{Result: &ops.Operation_Result_AppendFile{AppendFile: &ops.Operation_Ack{}}}
	}
	return &ops.Operation_Result{Result: &ops.Operation_Result_WriteFile{WriteFile: &ops.Operation_Ack{}}}
}

func fileContent(f *system.File) []byte {
	if c := f.GetContent(); c != nil {
		if b := c.GetBytes(); len(b) > 0 {
			return b
		}
		return []byte(c.GetText())
	}
	return nil
}
