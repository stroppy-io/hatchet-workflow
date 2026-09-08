package activities

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.temporal.io/sdk/activity"

	"github.com/graphene-ci/pipeline/pkg/machine"
	"github.com/graphene-ci/pipeline/pkg/obs"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// NameRunSegment is the wire name of RunSegment.
const NameRunSegment = "stroppy.segment.run"

// Environment keys of a RunSpec workload the runner interprets itself; every
// other key is handed to stroppy as a script env variable.
//
// doc: `stroppy help config-file` — drivers.0 {driverType, url,
// defaultInsertMethod, bulkSize}; env map; steps; k6Args.
const (
	EnvDriverURL          = "STROPPY_URL"
	EnvDriverType         = "STROPPY_DRIVER_TYPE"
	EnvDriverInsertMethod = "STROPPY_INSERT_METHOD"
	EnvOTLPHeaders        = "STROPPY_OTLP_HEADERS"
	// summaryFile is the k6 end-of-test summary the runner reads back.
	summaryFile = "summary.json"
	configFile  = "stroppy-config.json"
	// containerWorkspace is where the segment directory is mounted inside
	// the stroppy container.
	containerWorkspace = "/workspace"
)

// RunSegmentRequest runs one workload segment on the runner machine.
type RunSegmentRequest struct {
	RunID string `json:"run_id"`
	// Segment is the decoded workload.segment value.
	Segment spec.Segment `json:"segment"`
	// Index orders the segment inside the workload (directory name).
	Index int `json:"index"`
	// Image is the stroppy docker image.
	Image string `json:"image"`
	// Env is the workload-level environment (driver url/type + script env).
	Env map[string]string `json:"env,omitempty"`
	// OTLPEndpoint receives stroppy metrics; empty disables the exporter.
	OTLPEndpoint string `json:"otlp_endpoint,omitempty"`
	// Labels are stamped on the run's metrics as stroppy metadata.
	Labels map[string]string `json:"labels,omitempty"`
	// RegistrySecret names the registry login for a private stroppy image.
	RegistrySecret string `json:"registry_secret,omitempty"`
}

// RunSegmentResult is the segment's outcome plus where its artifacts are.
type RunSegmentResult struct {
	Result spec.SegmentResult `json:"result"`
	// SummaryPath is the k6 summary JSON on the machine (for the artifact).
	SummaryPath string `json:"summary_path,omitempty"`
	// LogPath is the full stroppy output on the machine.
	LogPath string `json:"log_path,omitempty"`
	// ExitCode of the stroppy container.
	ExitCode int `json:"exit_code"`
}

// RunSegment runs ON THE RUNNER MACHINE's agent container: writes the
// segment's config and files into the run workspace, starts stroppy as a
// container with the workspace mounted, streams its output to obs line by
// line, waits for exit and parses the k6 summary. One-shot by nature
// (AtMostOnce at the call site): a second execution would load data twice.
func RunSegment(ctx context.Context, req RunSegmentRequest) (RunSegmentResult, error) {
	seg := req.Segment
	started := time.Now().UTC()
	dir, err := segmentDir(req)
	if err != nil {
		return RunSegmentResult{}, err
	}
	if err := writeSegmentInputs(dir, req); err != nil {
		return RunSegmentResult{}, err
	}
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return RunSegmentResult{}, err
	}
	defer func() { _ = cli.Close() }()
	if _, err := PullImage(ctx, PullImageRequest{Image: req.Image, RegistrySecret: req.RegistrySecret}); err != nil {
		return RunSegmentResult{}, err
	}

	name := "stroppy-" + segmentSlug(req)
	// A leftover container of the same name (previous attempt that died
	// before reporting) is removed: the segment is one-shot, its state
	// unknowable — the workflow decides what a re-run means, not we.
	if err := cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true}); err != nil && !cerrdefs.IsNotFound(err) {
		obs.Warn(ctx, "stale stroppy container not removed", obs.Str("container", name), obs.Err(err))
	}
	created, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      req.Image,
		Cmd:        stroppyArgs(seg),
		Env:        stroppyEnv(req),
		WorkingDir: containerWorkspace,
		Labels:     map[string]string{"stroppy-run": req.RunID, "stroppy-segment": seg.Name},
	}, &container.HostConfig{
		NetworkMode: "host",
		Mounts:      []mount.Mount{{Type: mount.TypeBind, Source: dir, Target: containerWorkspace}},
		LogConfig:   container.LogConfig{Type: "json-file", Config: map[string]string{"max-size": "50m", "max-file": "3"}},
	}, nil, nil, name)
	if err != nil {
		return RunSegmentResult{}, fmt.Errorf("create stroppy container: %w", err)
	}
	if err := cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return RunSegmentResult{}, fmt.Errorf("start stroppy container: %w", err)
	}
	obs.Info(ctx, "segment started", obs.Str("segment", seg.Name), obs.Str("script", seg.Script))

	logPath := filepath.Join(dir, "stroppy.log")
	exitCode, streamErr := streamAndWait(ctx, cli, created.ID, logPath, seg.Name)
	finished := time.Now().UTC()
	res := RunSegmentResult{
		Result:   spec.SegmentResult{Name: seg.Name, StartedAt: started, FinishedAt: finished},
		LogPath:  logPath,
		ExitCode: exitCode,
	}
	summaryPath := filepath.Join(dir, summaryFile)
	if metrics, err := ParseK6Summary(summaryPath); err == nil {
		res.Result.Metrics = metrics
		res.SummaryPath = summaryPath
	} else {
		obs.Warn(ctx, "no k6 summary", obs.Str("segment", seg.Name), obs.Err(err))
	}
	switch {
	case streamErr != nil && errors.Is(streamErr, context.Canceled):
		res.Result.Status = spec.SegmentCancelled
		res.Result.Error = streamErr.Error()
	case streamErr != nil:
		res.Result.Status = spec.SegmentFailed
		res.Result.Error = streamErr.Error()
	case exitCode != 0:
		res.Result.Status = spec.SegmentFailed
		res.Result.Error = fmt.Sprintf("stroppy exited with %d: %s", exitCode, tail(lastLines(logPath, 20), errTailBytes))
	default:
		res.Result.Status = spec.SegmentCompleted
	}
	return res, nil
}

// segmentDir is the per-segment directory inside the machine's run
// workspace: the same path on the machine, in this container and for the
// docker daemon (bind-mount source).
func segmentDir(req RunSegmentRequest) (string, error) {
	ws := machine.Workspace()
	if ws == "" {
		return "", fmt.Errorf("run segment: no %s — not running in an agent container", machine.EnvWorkspace)
	}
	dir := filepath.Join(ws, "stroppy", segmentSlug(req))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func segmentSlug(req RunSegmentRequest) string {
	return fmt.Sprintf("%02d-%s", req.Index, strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, strings.ToLower(req.Segment.Name)))
}

// writeSegmentInputs writes stroppy-config.json and the segment files.
func writeSegmentInputs(dir string, req RunSegmentRequest) error {
	cfg, err := json.MarshalIndent(StroppyConfig(req), "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, configFile), cfg, 0o644); err != nil { //nolint:gosec // read by the stroppy container's user, not a secret
		return err
	}
	for _, f := range req.Segment.Files {
		if f.Content == "" {
			continue
		}
		name := filepath.Base(f.Name)
		if name == "" || name == "." {
			return fmt.Errorf("segment file: bad name %q", f.Name)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(f.Content), 0o644); err != nil { //nolint:gosec // workload file for the stroppy container
			return err
		}
	}
	return nil
}

// StroppyConfig builds the stroppy config file of a segment — the driver
// from the workload env, the script env, steps and the k6 arguments.
//
// doc: `stroppy help config-file`.
func StroppyConfig(req RunSegmentRequest) map[string]any {
	seg := req.Segment
	driver := map[string]any{}
	env := map[string]string{}
	for k, v := range req.Env {
		switch k {
		case EnvDriverURL:
			driver["url"] = v
		case EnvDriverType:
			driver["driverType"] = v
		case EnvDriverInsertMethod:
			driver["defaultInsertMethod"] = v
		case EnvOTLPHeaders:
		default:
			env[strings.ToUpper(k)] = v
		}
	}
	if seg.Params.InsertMethod != "" {
		driver["defaultInsertMethod"] = seg.Params.InsertMethod
	}
	if seg.Params.BulkSize > 0 {
		driver["bulkSize"] = seg.Params.BulkSize
	}
	if seg.Params.PoolSize > 0 {
		env["POOL_SIZE"] = strconv.Itoa(seg.Params.PoolSize)
	}
	if seg.Params.ScaleFactor > 0 {
		env["SCALE_FACTOR"] = strconv.Itoa(seg.Params.ScaleFactor)
	}
	for k, v := range seg.Params.Env {
		env[strings.ToUpper(k)] = v
	}
	global := map[string]any{
		"runId":    req.RunID,
		"metadata": req.Labels,
		"logger":   map[string]any{"logLevel": "LOG_LEVEL_INFO", "logMode": "LOG_MODE_PRODUCTION"},
	}
	cfg := map[string]any{
		"version": "1",
		"script":  seg.Script,
		"global":  global,
		"drivers": map[string]any{"0": driver},
		"env":     env,
		"k6Args":  K6Args(seg),
	}
	if req.OTLPEndpoint != "" {
		global["exporter"] = map[string]any{
			"name":       "otlp",
			"otlpExport": otlpExport(req.OTLPEndpoint, req.Env[EnvOTLPHeaders]),
		}
	}
	if len(seg.Params.Steps) > 0 {
		cfg["steps"] = seg.Params.Steps
	}
	if len(seg.Params.NoSteps) > 0 {
		cfg["noSteps"] = seg.Params.NoSteps
	}
	return cfg
}

// otlpExport maps an endpoint URL to stroppy's OtlpExport: http(s)://…
// goes over OTLP/HTTP, anything else is treated as a gRPC host:port.
func otlpExport(endpoint, headers string) map[string]any {
	out := map[string]any{}
	switch {
	case strings.HasPrefix(endpoint, "http://"):
		out["otlpHttpEndpoint"] = strings.TrimPrefix(endpoint, "http://")
		out["otlpEndpointInsecure"] = true
	case strings.HasPrefix(endpoint, "https://"):
		out["otlpHttpEndpoint"] = strings.TrimPrefix(endpoint, "https://")
	default:
		out["otlpGrpcEndpoint"] = endpoint
	}
	if headers != "" {
		out["otlpHeaders"] = headers
	}
	return out
}

// K6Args renders the segment execution bounds as k6 flags.
func K6Args(seg spec.Segment) []string {
	args := []string{"--vus", strconv.Itoa(max(seg.Execution.VUs, 1))}
	switch seg.Execution.Limit.Kind {
	case "iterations":
		args = append(args, "--iterations", strconv.FormatInt(seg.Execution.Limit.Iterations, 10))
	default:
		args = append(args, "--duration", seg.Execution.Limit.Duration.Std().String())
	}
	if seg.Execution.Quiet {
		args = append(args, "--quiet")
	}
	if seg.Execution.NoThresholds {
		args = append(args, "--no-thresholds")
	}
	args = append(args, "--summary-export", containerWorkspace+"/"+summaryFile)
	args = append(args, seg.Execution.ExtraArgs...)
	return args
}

// stroppyArgs is the container command: `stroppy run -f <config> <script>`
// — the image's entrypoint is the stroppy binary.
func stroppyArgs(seg spec.Segment) []string {
	return []string{"run", "-f", containerWorkspace + "/" + configFile, seg.Script}
}

// stroppyEnv is the container environment: only what the config file
// cannot carry (real env wins over the file for script variables, so
// nothing from the workload goes here except logging).
func stroppyEnv(_ RunSegmentRequest) []string {
	return []string{"LOG_MODE=production", "LOG_LEVEL=info"}
}

// streamAndWait copies the container's output to obs and the log file
// until it exits, heartbeating on every line. Returns the exit code.
func streamAndWait(ctx context.Context, cli *dockerclient.Client, id, logPath, segment string) (int, error) {
	logs, err := cli.ContainerLogs(ctx, id, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return -1, fmt.Errorf("container logs: %w", err)
	}
	defer func() { _ = logs.Close() }()
	f, err := os.Create(logPath)
	if err != nil {
		return -1, err
	}
	defer func() { _ = f.Close() }()

	pr, pw := io.Pipe()
	go func() {
		_, cerr := stdcopy.StdCopy(pw, pw, logs)
		_ = pw.CloseWithError(cerr)
	}()
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	lines := 0
	for sc.Scan() {
		line := sc.Text()
		if _, err := f.WriteString(line + "\n"); err != nil {
			obs.Warn(ctx, "log file write failed", obs.Err(err))
		}
		obs.Info(ctx, line, obs.Str("segment", segment), obs.Str("stream", "stroppy"))
		lines++
		if lines%50 == 0 && activity.IsActivity(ctx) {
			activity.RecordHeartbeat(ctx, fmt.Sprintf("%s: %d lines", segment, lines))
		}
	}
	waitCh, errCh := cli.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case res := <-waitCh:
		if res.Error != nil {
			return int(res.StatusCode), errors.New(res.Error.Message)
		}
		return int(res.StatusCode), nil
	case err := <-errCh:
		return -1, err
	case <-ctx.Done():
		return -1, ctx.Err()
	}
}

// ParseK6Summary reads a k6 --summary-export file into metric values:
// every metric × statistic becomes "<metric>_<stat>" (p(95) → p95). Both
// the nested {"values": {...}} and the flat legacy shapes are accepted.
func ParseK6Summary(path string) (map[string]spec.MetricValue, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseK6Summary(raw)
}

func parseK6Summary(raw []byte) (map[string]spec.MetricValue, error) {
	var doc struct {
		Metrics map[string]json.RawMessage `json:"metrics"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("k6 summary: %w", err)
	}
	out := map[string]spec.MetricValue{}
	for name, body := range doc.Metrics {
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			continue
		}
		values := m
		if v, ok := m["values"].(map[string]any); ok {
			values = v
		}
		for stat, val := range values {
			f, ok := val.(float64)
			if !ok {
				continue
			}
			key := metricKey(name, stat)
			mv := spec.MetricValue{Value: f}
			out[key] = mv
		}
	}
	return out, nil
}

// metricKey joins a k6 metric and statistic into one key: iteration_duration
// + p(95) → iteration_duration_p95.
func metricKey(name, stat string) string {
	stat = strings.NewReplacer("(", "", ")", "", ".", "_").Replace(stat)
	return strings.ToLower(name + "_" + stat)
}

// Headline picks the summary numbers of a run from segment metrics: the
// last workload segment's iterations rate as TPS and iteration_duration
// percentiles as latency.
func Headline(segments []spec.SegmentResult) spec.Summary {
	var s spec.Summary
	for i := len(segments) - 1; i >= 0; i-- {
		m := segments[i].Metrics
		if m == nil {
			continue
		}
		if v, ok := m["iterations_rate"]; ok {
			s.TPS = v.Value
			s.LatencyP50Ms = m["iteration_duration_med"].Value
			s.LatencyP95Ms = m["iteration_duration_p95"].Value
			s.LatencyP99Ms = m["iteration_duration_p99"].Value
			if e, ok := m["dropped_iterations_count"]; ok {
				s.Errors = int64(e.Value)
			}
			break
		}
	}
	return s
}

// MergeMetrics folds segment metrics into run-level metrics, prefixed by
// the segment name so two segments never collide.
func MergeMetrics(segments []spec.SegmentResult) map[string]spec.MetricValue {
	out := map[string]spec.MetricValue{}
	for _, seg := range segments {
		keys := make([]string, 0, len(seg.Metrics))
		for k := range seg.Metrics {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out[seg.Name+"."+k] = seg.Metrics[k]
		}
	}
	return out
}

func lastLines(path string, n int) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
