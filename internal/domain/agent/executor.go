package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// LogCallback is invoked for every output line produced by shell commands.
// commandID identifies the originating command; action is the command's Label
// (e.g. "install_postgres"); line is the raw text; stream is "stdout".
type LogCallback func(commandID, action, line, stream string)

// Executor runs agent primitive commands on the local machine.
//
// The agent is intentionally dumb: it knows how to run a shell script, write a
// file, start a tracked background process, and shut down. It has NO knowledge
// of databases, exporters, package versions, or config formats — all of that is
// composed on the server and arrives as opaque scripts/file-bodies.
//
// Long-running processes (DB servers, exporters, vmagent) are tracked in a pool
// and killed on Shutdown(). Their stdout/stderr are streamed via logCallback.
type Executor struct {
	// aptMu serializes "exclusive" run_cmd commands (apt/dpkg) so concurrent
	// commands on the same agent don't fight over the dpkg lock.
	aptMu sync.Mutex

	logMu         sync.RWMutex
	logCallback   LogCallback
	currentCmd    string
	currentAction string

	// Process pool — tracked background processes killed on shutdown.
	procMu sync.Mutex
	procs  []*managedProc
}

type managedProc struct {
	name string
	cmd  *exec.Cmd
	stop func()
}

// NewExecutor returns a new Executor.
func NewExecutor() *Executor { return &Executor{} }

// Shutdown kills all tracked background processes.
func (e *Executor) Shutdown() {
	e.procMu.Lock()
	defer e.procMu.Unlock()
	for _, p := range e.procs {
		if p.cmd.Process != nil {
			p.cmd.Process.Kill()
		}
		if p.stop != nil {
			p.stop()
		}
	}
	e.procs = nil
}

// startDaemon launches a long-running process, tracks it in the pool, and
// streams its output through logCallback. Returns immediately.
func (e *Executor) startDaemon(name, binPath string, env map[string]string, args ...string) error {
	cmd := exec.Command(binPath, args...)
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		pw.Close()
		return fmt.Errorf("start %s: %w", name, err)
	}

	// Stream output in background.
	go func() {
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			e.emitLine(scanner.Text())
		}
	}()

	// Wait for process in background — when it dies, close pipe.
	stopCh := make(chan struct{})
	go func() {
		cmd.Wait()
		pw.Close()
		close(stopCh)
	}()

	e.procMu.Lock()
	e.procs = append(e.procs, &managedProc{
		name: name,
		cmd:  cmd,
		stop: func() { <-stopCh },
	})
	e.procMu.Unlock()

	return nil
}

// SetLogCallback registers a callback that receives every shell output line in real-time.
func (e *Executor) SetLogCallback(cb LogCallback) {
	e.logMu.Lock()
	defer e.logMu.Unlock()
	e.logCallback = cb
}

// setCurrentCommand stores the currently executing command ID and label for log correlation.
func (e *Executor) setCurrentCommand(id, label string) {
	e.logMu.Lock()
	defer e.logMu.Unlock()
	e.currentCmd = id
	e.currentAction = label
}

// emitLine sends a single output line to the registered callback (if any).
func (e *Executor) emitLine(line string) {
	// Always print to stderr so `docker logs` captures everything.
	fmt.Fprintln(os.Stderr, line)

	e.logMu.RLock()
	cb := e.logCallback
	cmdID := e.currentCmd
	action := e.currentAction
	e.logMu.RUnlock()
	if cb != nil {
		cb(cmdID, action, line, "stdout")
	}
}

// Run executes a primitive Command and returns a Report.
func (e *Executor) Run(ctx context.Context, cmd Command) Report {
	report := Report{CommandID: cmd.ID, Status: ReportCompleted}

	// Store command ID and label so streamed log lines are correlated.
	label := cmd.Label
	if label == "" {
		label = string(cmd.Action)
	}
	e.setCurrentCommand(cmd.ID, label)
	defer e.setCurrentCommand("", "")

	var err error
	switch cmd.Action {
	case ActionRunCmd:
		err = e.runCmd(ctx, cmd)
	case ActionWriteFile:
		err = e.writeFile(cmd)
	case ActionStartDaemon:
		err = e.startDaemonCmd(cmd)
	case ActionShutdown:
		e.Shutdown()
	default:
		err = fmt.Errorf("unknown action: %s", cmd.Action)
	}

	if err != nil {
		report.Status = ReportFailed
		report.Error = err.Error()
	}
	return report
}

// runCmd handles ActionRunCmd. When Exclusive is set the apt mutex is held for
// the duration so concurrent apt/dpkg commands serialize. The script's output
// streams as logs (no need to echo it back in the report).
func (e *Executor) runCmd(ctx context.Context, cmd Command) error {
	var cfg RunCmdConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}
	if cfg.Exclusive {
		e.aptMu.Lock()
		defer e.aptMu.Unlock()
	}
	_, err := e.shell(ctx, cfg.Script)
	return err
}

// writeFile handles ActionWriteFile.
func (e *Executor) writeFile(cmd Command) error {
	var cfg WriteFileConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}
	if cfg.Path == "" {
		return fmt.Errorf("write_file: empty path")
	}

	mkdir := cfg.MkdirParents == nil || *cfg.MkdirParents
	if mkdir {
		if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
			return fmt.Errorf("write_file: mkdir %s: %w", filepath.Dir(cfg.Path), err)
		}
	}

	mode := os.FileMode(0o644)
	if cfg.Mode != 0 {
		mode = os.FileMode(cfg.Mode)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if cfg.Append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	f, err := os.OpenFile(cfg.Path, flags, mode)
	if err != nil {
		return fmt.Errorf("write_file: open %s: %w", cfg.Path, err)
	}
	if _, err := f.WriteString(cfg.Content); err != nil {
		f.Close()
		return fmt.Errorf("write_file: write %s: %w", cfg.Path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("write_file: close %s: %w", cfg.Path, err)
	}
	// Ensure mode is applied even when the file pre-existed.
	if err := os.Chmod(cfg.Path, mode); err != nil {
		return fmt.Errorf("write_file: chmod %s: %w", cfg.Path, err)
	}

	if cfg.Owner != "" {
		if _, err := e.shell(context.Background(), fmt.Sprintf("chown %s %q", cfg.Owner, cfg.Path)); err != nil {
			return fmt.Errorf("write_file: chown %s: %w", cfg.Path, err)
		}
	}

	e.emitLine(fmt.Sprintf("wrote %s (%d bytes)", cfg.Path, len(cfg.Content)))
	return nil
}

// startDaemonCmd handles ActionStartDaemon.
func (e *Executor) startDaemonCmd(cmd Command) error {
	var cfg StartDaemonConfig
	if err := parseConfig(cmd, &cfg); err != nil {
		return err
	}
	if cfg.Bin == "" {
		return fmt.Errorf("start_daemon: empty bin")
	}
	name := cfg.Name
	if name == "" {
		name = filepath.Base(cfg.Bin)
	}
	if err := e.startDaemon(name, cfg.Bin, cfg.Env, cfg.Args...); err != nil {
		return err
	}
	e.emitLine(fmt.Sprintf("started daemon %s (%s)", name, cfg.Bin))
	return nil
}

// shell runs a bash script, streaming each output line to the executor's
// logCallback in real-time while also accumulating the full output.
func (e *Executor) shell(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", script)

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	var fullOutput strings.Builder

	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(pr)
		// Allow lines up to 1 MB (apt-get progress, curl, etc.).
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			fullOutput.WriteString(line + "\n")
			e.emitLine(line)
		}
	}()

	if err := cmd.Start(); err != nil {
		pw.Close()
		return "", err
	}

	cmdErr := cmd.Wait()
	pw.Close()
	<-done

	output := fullOutput.String()
	if cmdErr != nil {
		return output, fmt.Errorf("%w: %s", cmdErr, output)
	}
	return output, nil
}

// parseConfig marshals cmd.Config through JSON into the target struct.
func parseConfig(cmd Command, target any) error {
	data, err := json.Marshal(cmd.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal config into %T: %w", target, err)
	}
	return nil
}
