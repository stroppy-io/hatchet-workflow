// Package agent implements the stroppy cloud agent: a process that runs inside
// a deployed container (emulating a cloud VM 1:1), connects to Temporal as a
// worker, and executes the AgentCommandService activities locally.
package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// heartbeater abstracts activity.RecordHeartbeat so the activities can be unit
// tested with a plain context.Background() (no Temporal runtime needed).
type heartbeater func(ctx context.Context, details ...any)

// Activities implements workflow.AgentCommandServiceActivities. It runs every
// command/file/dir operation directly on the agent host.
type Activities struct {
	// cacheDir is where files fetched by checksum are cached.
	cacheDir string
	// logger is used to stream command stdout/stderr and lifecycle events.
	logger *slog.Logger
	// logSink ships command stdout/stderr to the control-plane log ingest.
	logSink LogSink
	// heartbeat records a Temporal heartbeat; overridable for tests.
	heartbeat heartbeater
	// httpClient downloads referenced files; overridable for tests.
	httpClient *http.Client
}

// compile-time assertion that *Activities satisfies the generated interface.
var _ workflow.AgentCommandServiceActivities = (*Activities)(nil)

// Option configures an Activities instance.
type Option func(*Activities)

// WithCacheDir sets the directory used to cache fetched files by checksum.
func WithCacheDir(dir string) Option {
	return func(a *Activities) { a.cacheDir = dir }
}

// WithLogger sets the logger used for stream/lifecycle output.
func WithLogger(l *slog.Logger) Option {
	return func(a *Activities) { a.logger = l }
}

// WithLogSink sets the sink used to ship command stdout/stderr.
func WithLogSink(s LogSink) Option {
	return func(a *Activities) { a.logSink = s }
}

// WithHeartbeater overrides the heartbeat function (used by tests).
func WithHeartbeater(h heartbeater) Option {
	return func(a *Activities) { a.heartbeat = h }
}

// WithHTTPClient overrides the HTTP client used by FetchFile (used by tests).
func WithHTTPClient(c *http.Client) Option {
	return func(a *Activities) { a.httpClient = c }
}

// NewActivities constructs an Activities with sensible defaults. By default it
// uses activity.RecordHeartbeat (imported via the temporal SDK) so that long
// running commands keep their activity alive.
func NewActivities(opts ...Option) *Activities {
	a := &Activities{
		cacheDir:   filepath.Join(os.TempDir(), "stroppy-agent-cache"),
		logger:     slog.Default(),
		heartbeat:  recordHeartbeat,
		httpClient: &http.Client{Timeout: 0}, // no client-wide timeout; downloads may be large
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// EnsureAgentOnlineActivity returns nil: the agent is online by virtue of the
// worker polling this task queue. We perform a trivial readiness check (we can
// reach the filesystem) to fail fast on a broken host.
func (a *Activities) EnsureAgentOnlineActivity(ctx context.Context) error {
	if _, err := os.Stat(os.TempDir()); err != nil {
		return fmt.Errorf("agent readiness check failed: %w", err)
	}
	a.logger.InfoContext(ctx, "agent online")
	return nil
}

// CreateDirActivity creates a directory with mkdir -p semantics, honoring the
// requested path and mode.
func (a *Activities) CreateDirActivity(ctx context.Context, req *common.Dir) error {
	info := req.GetInfo()
	if info.GetPath() == "" {
		return fmt.Errorf("CreateDir: empty path")
	}
	mode := dirMode(info.GetMode())
	if err := os.MkdirAll(info.GetPath(), mode); err != nil {
		return fmt.Errorf("CreateDir %q: %w", info.GetPath(), err)
	}
	// MkdirAll respects umask; force the exact mode on the leaf.
	if info.GetMode() != 0 {
		if err := os.Chmod(info.GetPath(), mode); err != nil {
			return fmt.Errorf("CreateDir chmod %q: %w", info.GetPath(), err)
		}
	}
	a.logger.InfoContext(ctx, "created dir", "path", info.GetPath(), "mode", mode.String())
	return nil
}

// CreateTempDirActivity creates a fresh temporary directory and returns its
// resolved path as a Dir.
func (a *Activities) CreateTempDirActivity(ctx context.Context, req *common.Dir_Temp) (*common.Dir, error) {
	base := req.GetBase()
	if base != "" {
		if err := os.MkdirAll(base, 0o755); err != nil {
			return nil, fmt.Errorf("CreateTempDir base %q: %w", base, err)
		}
	}
	pattern := req.GetPattern()
	if pattern == "" {
		pattern = "stroppy-*"
	}
	path, err := os.MkdirTemp(base, pattern)
	if err != nil {
		return nil, fmt.Errorf("CreateTempDir: %w", err)
	}
	if req.GetMode() != 0 {
		if err := os.Chmod(path, dirMode(req.GetMode())); err != nil {
			return nil, fmt.Errorf("CreateTempDir chmod %q: %w", path, err)
		}
	}
	a.logger.InfoContext(ctx, "created temp dir", "path", path)
	return &common.Dir{
		Info: &common.Dir_Info{
			Path:  path,
			Mode:  req.GetMode(),
			Owner: req.GetOwner(),
			Group: req.GetGroup(),
		},
		CreateParents: true,
	}, nil
}

// WriteFileActivity writes File content (text or bytes) to info.path with the
// requested mode, creating parent directories as needed.
func (a *Activities) WriteFileActivity(ctx context.Context, req *common.File) error {
	info := req.GetInfo()
	if info.GetPath() == "" {
		return fmt.Errorf("WriteFile: empty path")
	}
	var data []byte
	switch {
	case req.GetText() != "":
		data = []byte(req.GetText())
	case req.GetBytes() != nil:
		data = req.GetBytes()
	case req.GetAsRef() != nil:
		return fmt.Errorf("WriteFile %q: content is a reference; use FetchFile", info.GetPath())
	default:
		// empty file is valid
		data = []byte{}
	}

	if err := os.MkdirAll(filepath.Dir(info.GetPath()), 0o755); err != nil {
		return fmt.Errorf("WriteFile mkdir parent of %q: %w", info.GetPath(), err)
	}

	mode := fileMode(info.GetMode())
	flag := os.O_CREATE | os.O_WRONLY
	if req.GetAppend() {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(info.GetPath(), flag, mode)
	if err != nil {
		return fmt.Errorf("WriteFile open %q: %w", info.GetPath(), err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("WriteFile write %q: %w", info.GetPath(), err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("WriteFile close %q: %w", info.GetPath(), err)
	}
	if info.GetMode() != 0 {
		if err := os.Chmod(info.GetPath(), mode); err != nil {
			return fmt.Errorf("WriteFile chmod %q: %w", info.GetPath(), err)
		}
	}
	a.logger.InfoContext(ctx, "wrote file", "path", info.GetPath(), "bytes", len(data))
	return nil
}

// CallCmdActivity runs a command (argv or shell script) capturing
// stdout/stderr/exit_code/timed_out/elapsed. It heartbeats on a ticker so long
// running commands (e.g. the stroppy load) keep their activity alive.
func (a *Activities) CallCmdActivity(ctx context.Context, req *common.Cmd) (*common.Cmd_Result, error) {
	spec := req.GetSpec()
	if spec == nil {
		return nil, fmt.Errorf("CallCmd: nil spec")
	}

	// Build the *exec.Cmd from either argv or a shell script.
	runCtx := ctx
	var cancel context.CancelFunc
	if d := spec.GetTimeout().AsDuration(); d > 0 {
		runCtx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}

	var cmd *exec.Cmd
	switch {
	case spec.GetArgv() != nil:
		args := spec.GetArgv().GetArgs()
		if len(args) == 0 {
			return nil, fmt.Errorf("CallCmd: empty argv")
		}
		cmd = exec.CommandContext(runCtx, args[0], args[1:]...) //nolint:gosec // executor by design
	case spec.GetScript() != nil:
		shell := spec.GetScript().GetShell()
		if shell == "" {
			shell = "/bin/sh"
		}
		cmd = exec.CommandContext(runCtx, shell, "-c", spec.GetScript().GetText()) //nolint:gosec // executor by design
	default:
		return nil, fmt.Errorf("CallCmd: neither argv nor script set")
	}

	cmd.Dir = spec.GetCwd()
	if env := spec.GetEnv(); len(env) > 0 {
		merged := os.Environ()
		for k, v := range env {
			merged = append(merged, k+"="+v)
		}
		cmd.Env = merged
	}
	if stdin := spec.GetStdin(); len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	// stdout/stderr handling. We always capture (CAPTURE is the useful default)
	// and additionally stream to the agent logger + AgentLogService when the
	// workflow stamped log correlation env vars on the command.
	streams := spec.GetStreams()
	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutLog := a.streamLogger(ctx, spec.GetEnv(), "stdout")
	stderrLog := a.streamLogger(ctx, spec.GetEnv(), "stderr")
	cmd.Stdout = streamWriter(&stdoutBuf, stdoutLog, streamMode(streams.GetStdout()))
	if streamMode(streams.GetStderr()) == common.Cmd_Streams_MODE_STDERR_TO_STDOUT {
		cmd.Stderr = cmd.Stdout
	} else {
		cmd.Stderr = streamWriter(&stderrBuf, stderrLog, streamMode(streams.GetStderr()))
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("CallCmd start: %w", err)
	}

	// Heartbeat ticker keeps the activity alive while the command runs.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var waitErr error
loop:
	for {
		select {
		case <-ticker.C:
			a.heartbeat(ctx, "running", time.Since(start).String())
		case waitErr = <-done:
			break loop
		}
	}
	elapsed := time.Since(start)
	if stdoutLog != nil {
		stdoutLog.Flush()
	}
	if streamMode(streams.GetStderr()) != common.Cmd_Streams_MODE_STDERR_TO_STDOUT {
		if stderrLog != nil {
			stderrLog.Flush()
		}
	}

	timedOut := runCtx.Err() == context.DeadlineExceeded

	result := &common.Cmd_Result{
		Stdout:   stdoutBuf.Bytes(),
		Stderr:   stderrBuf.Bytes(),
		TimedOut: timedOut,
		Elapsed:  durationpb.New(elapsed),
	}
	result.ExitCode = int32(exitCode(waitErr))

	a.logger.InfoContext(ctx, "command finished",
		"exit_code", result.ExitCode,
		"timed_out", timedOut,
		"elapsed", elapsed.String(),
	)

	// A timeout or non-zero unexpected exit is reported via the result, not an
	// activity error, so the workflow can inspect exit_code/timed_out. We only
	// surface a Go error if the result itself could not be produced.
	return result, nil
}

// FetchFileActivity downloads File.AsRef.uri (http/https) to File.info.path,
// verifying sha256 against AsRef.checksum when set, caching by checksum so a
// repeated fetch is a no-op copy from cache. Binaries (mode with +x) are made
// executable.
func (a *Activities) FetchFileActivity(ctx context.Context, req *common.File) error {
	info := req.GetInfo()
	if info.GetPath() == "" {
		return fmt.Errorf("FetchFile: empty path")
	}
	ref := req.GetAsRef()
	if ref == nil || ref.GetUri() == "" {
		return fmt.Errorf("FetchFile %q: missing as_ref.uri", info.GetPath())
	}

	if err := os.MkdirAll(filepath.Dir(info.GetPath()), 0o755); err != nil {
		return fmt.Errorf("FetchFile mkdir parent of %q: %w", info.GetPath(), err)
	}
	mode := fileMode(info.GetMode())

	// Try the cache first (only meaningful when a checksum is supplied).
	checksum := ref.GetChecksum()
	if checksum != "" {
		cached := filepath.Join(a.cacheDir, checksum)
		if _, err := os.Stat(cached); err == nil {
			a.logger.InfoContext(ctx, "fetch: cache hit", "checksum", checksum, "path", info.GetPath())
			if err := copyFile(cached, info.GetPath(), mode); err != nil {
				return fmt.Errorf("FetchFile copy from cache: %w", err)
			}
			return nil
		}
	}

	// Download to a temp file (heartbeating as bytes arrive), then verify.
	if err := os.MkdirAll(a.cacheDir, 0o755); err != nil {
		return fmt.Errorf("FetchFile mkdir cache: %w", err)
	}
	tmp, err := os.CreateTemp(a.cacheDir, "download-*")
	if err != nil {
		return fmt.Errorf("FetchFile temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // removed unless renamed into the cache

	a.logger.InfoContext(ctx, "fetch: downloading", "uri", ref.GetUri(), "path", info.GetPath())
	gotSum, err := a.download(ctx, ref.GetUri(), tmp)
	_ = tmp.Close()
	if err != nil {
		return fmt.Errorf("FetchFile download %q: %w", ref.GetUri(), err)
	}

	if checksum != "" && gotSum != checksum {
		return fmt.Errorf("FetchFile checksum mismatch for %q: want %s got %s", ref.GetUri(), checksum, gotSum)
	}

	// Populate the cache (keyed by the real checksum) and place the file.
	cacheKey := checksum
	if cacheKey == "" {
		cacheKey = gotSum
	}
	cached := filepath.Join(a.cacheDir, cacheKey)
	if err := os.Rename(tmpName, cached); err != nil {
		// Rename may fail across filesystems; fall back to copy.
		if err := copyFile(tmpName, cached, 0o644); err != nil {
			return fmt.Errorf("FetchFile populate cache: %w", err)
		}
	}
	if err := copyFile(cached, info.GetPath(), mode); err != nil {
		return fmt.Errorf("FetchFile place file: %w", err)
	}
	a.logger.InfoContext(ctx, "fetch: done", "checksum", gotSum, "path", info.GetPath())
	return nil
}

// download streams the URL body into w, returning the hex sha256 of the bytes.
// It heartbeats periodically so large downloads keep the activity alive.
func (a *Activities) download(ctx context.Context, uri string, w io.Writer) (string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return "", err
	}
	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %s", resp.Status)
	}

	h := sha256.New()
	hb := &heartbeatWriter{ctx: ctx, hb: a.heartbeat, every: 5 * time.Second, last: time.Now()}
	if _, err := io.Copy(io.MultiWriter(w, h, hb), resp.Body); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// streamLogger returns an io.Writer that forwards process output lines to the
// local agent logger and the remote log sink when either is configured.
func (a *Activities) streamLogger(ctx context.Context, env map[string]string, stream string) *streamLogWriter {
	if a.logger == nil && a.logSink == nil {
		return nil
	}
	writers := make([]io.Writer, 0, 2)
	if a.logger != nil {
		writers = append(writers, &slogWriter{ctx: ctx, logger: a.logger, stream: stream})
	}
	if a.logSink != nil {
		logStream := monitorStream(stream)
		if logStream != monitor.Stream_STREAM_UNSPECIFIED {
			writers = append(writers, &logSinkWriter{
				ctx:     ctx,
				logger:  a.logger,
				sink:    a.logSink,
				context: commandLogContextFromEnv(env),
				stream:  logStream,
			})
		}
	}
	if len(writers) == 1 {
		return &streamLogWriter{writers: writers}
	}
	return &streamLogWriter{writers: writers}
}

// --- helpers -------------------------------------------------------------

func dirMode(m uint32) os.FileMode {
	if m == 0 {
		return 0o755
	}
	return os.FileMode(m)
}

func fileMode(m uint32) os.FileMode {
	if m == 0 {
		return 0o644
	}
	return os.FileMode(m)
}

func streamMode(m common.Cmd_Streams_Mode) common.Cmd_Streams_Mode {
	if m == common.Cmd_Streams_MODE_UNSPECIFIED {
		return common.Cmd_Streams_MODE_CAPTURE
	}
	return m
}

func monitorStream(stream string) monitor.Stream {
	switch stream {
	case "stdout":
		return monitor.Stream_STREAM_STDOUT
	case "stderr":
		return monitor.Stream_STREAM_STDERR
	default:
		return monitor.Stream_STREAM_UNSPECIFIED
	}
}

// streamWriter builds the destination for a process stream honoring its mode.
// capture writes into buf; inherit/stderr-to-stdout also tee to the logger;
// discard drops everything.
func streamWriter(buf *bytes.Buffer, log *streamLogWriter, mode common.Cmd_Streams_Mode) io.Writer {
	switch mode {
	case common.Cmd_Streams_MODE_DISCARD:
		return io.Discard
	case common.Cmd_Streams_MODE_INHERIT:
		if log != nil {
			return io.MultiWriter(buf, log)
		}
		return buf
	case common.Cmd_Streams_MODE_STDERR_TO_STDOUT, common.Cmd_Streams_MODE_CAPTURE, common.Cmd_Streams_MODE_UNSPECIFIED:
		if log != nil {
			return io.MultiWriter(buf, log)
		}
		return buf
	default:
		return buf
	}
}

// exitCode extracts a process exit code from the error returned by cmd.Wait.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

// copyFile copies src to dst, creating parents and setting mode.
func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

// slogWriter forwards process output to slog at debug level.
type slogWriter struct {
	ctx    context.Context
	logger *slog.Logger
	stream string
}

func (w *slogWriter) Write(p []byte) (int, error) {
	w.logger.DebugContext(w.ctx, "cmd output", "stream", w.stream, "data", string(bytes.TrimRight(p, "\n")))
	return len(p), nil
}

// heartbeatWriter records a heartbeat at most once per `every` interval as data
// flows through it. It always reports progress as bytes copied.
type heartbeatWriter struct {
	ctx     context.Context
	hb      heartbeater
	every   time.Duration
	last    time.Time
	written int64
}

func (h *heartbeatWriter) Write(p []byte) (int, error) {
	h.written += int64(len(p))
	if time.Since(h.last) >= h.every {
		h.hb(h.ctx, "downloaded", strconv.FormatInt(h.written, 10))
		h.last = time.Now()
	}
	return len(p), nil
}
