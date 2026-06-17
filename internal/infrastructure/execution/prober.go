package execution

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	stroppypb "github.com/stroppy-io/stroppy/pkg/common/proto/stroppy"
)

// StroppyProber restores the server-side stroppy "probe" capability that the
// monolith exposed at POST /api/v1/probe: it execs a server-local `stroppy
// probe` against a script + driver and returns the script metadata (available
// steps, declared env vars, SQL sections, driver defaults, pool size).
//
// It is a self-contained adapter: it owns the request/response shape (mirroring
// the monolith's probeRequest / stroppy's JSON probe output) so it does NOT
// depend on a Probe RPC — the regenerated connect API does not declare one yet
// (see the package docs + the wiring note in internal/app/run.go). Once a Probe
// RPC + request/response messages exist in the proto, a connect service can
// adapt to this prober with a thin field-by-field mapping; nothing else here
// needs to change.
//
// The stroppy binary is located the same way the monolith located it:
//   - STROPPY_PROBE_BIN env override (an explicit path) wins;
//   - otherwise the configured upstream (the same URL the gateway serves the
//     "stroppy" artifact from — Config.StroppyUpstream / STROPPY_UPSTREAM) is
//     downloaded once into a local cache dir and reused;
//   - a request-supplied version (a github release tag or "commit:<sha>")
//     overrides the upstream and is fetched from the pinned github release URL;
//   - failing all of those, a "stroppy" on PATH is used.
type StroppyProber struct {
	// upstream is the default stroppy binary source (Config.StroppyUpstream): a
	// tarball/raw-executable URL, or a local filesystem path. May be empty.
	upstream string
	// cacheDir is where downloaded binaries are cached (one subdir per version).
	cacheDir string

	mu sync.Mutex
}

// NewStroppyProber builds the prober.
//
//   - upstream is the default stroppy binary source, the same value wired into
//     the gateway's "stroppy" artifact (app.Config.StroppyUpstream /
//     STROPPY_UPSTREAM). It may be an http(s) URL (a release tarball or a raw
//     executable) or a local filesystem path. Empty is allowed: the prober then
//     relies on STROPPY_PROBE_BIN, a per-request version, or a PATH lookup.
//   - cacheDir is the directory downloaded binaries are cached under. Empty
//     falls back to <os.TempDir>/stroppy-cloud/stroppy-binaries.
func NewStroppyProber(upstream, cacheDir string) *StroppyProber {
	return &StroppyProber{
		upstream: strings.TrimSpace(upstream),
		cacheDir: strings.TrimSpace(cacheDir),
	}
}

// ProbeRequest is the input to Probe. It mirrors the monolith's probeRequest:
// the script (required), an optional second SQL argument, the driver type and
// its pool size / scale factor, plus env overrides and inline workload files.
type ProbeRequest struct {
	// Version selects the stroppy binary: a github release tag (e.g. "1.2.0") or
	// "commit:<sha>". Empty uses the configured upstream / PATH binary.
	Version string `json:"version,omitempty"`
	// Script is the stroppy script to introspect (e.g. "tpcc/procs", "tpcb/tx").
	Script string `json:"script"`
	// SQL is an optional second SQL argument.
	SQL string `json:"sql,omitempty"`
	// DriverType is the stroppy driver (e.g. "postgres", "mysql", "picodata",
	// "ydb"). Empty probes without a driver.
	DriverType string `json:"driver_type,omitempty"`
	// PoolSize, when > 0, sets the driver pool min/max conns and a POOL_SIZE env.
	PoolSize int `json:"pool_size,omitempty"`
	// ScaleFactor, when > 0, sets a SCALE_FACTOR env override. Fractional values
	// are valid (e.g. TPCH smoke runs use 0.01).
	ScaleFactor float64 `json:"scale_factor,omitempty"`
	// Env are extra env overrides for the probed script (keys uppercased).
	Env map[string]string `json:"env,omitempty"`
	// Files are inline workload files written next to the generated config so the
	// probed script can reference them by name.
	Files []ProbeWorkloadFile `json:"files,omitempty"`
}

// ProbeWorkloadFile is an inline workload file made available to the probe.
type ProbeWorkloadFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// ProbeResult is the parsed probe metadata. Metadata holds stroppy's structured
// JSON probe output (steps, declared env vars, SQL sections, driver defaults,
// pool size) decoded as-is; Human, when requested, holds the human-readable
// rendering of the same probe.
type ProbeResult struct {
	// Metadata is stroppy's `probe -o json` output, decoded structurally.
	Metadata map[string]any `json:"metadata"`
	// Human is stroppy's `probe -o human` output, populated only when
	// ProbeOptions.IncludeHuman is set and the human render succeeds.
	Human string `json:"human,omitempty"`
}

// ProbeOptions tune a Probe call.
type ProbeOptions struct {
	// IncludeHuman additionally runs `probe -o human` and fills ProbeResult.Human.
	IncludeHuman bool
}

// Probe execs `stroppy probe -f <config> -o json` against a generated minimal
// run config and parses the JSON metadata. It mirrors the monolith's
// executeStroppyProbe exactly: it builds a stroppypb.RunConfig (script, optional
// sql, driver with default URL + pool, env overrides incl. SCALE_FACTOR /
// POOL_SIZE), marshals it via protojson, writes it plus any inline workload
// files to a temp dir, resolves the stroppy binary, and runs the probe with
// stdout (the JSON payload) captured separately from stderr (logs).
//
// On a non-zero exit the captured stderr is returned as the error detail (the
// human-facing probe failure), so callers can surface stroppy's own diagnostic.
func (p *StroppyProber) Probe(ctx context.Context, req ProbeRequest, opts ProbeOptions) (*ProbeResult, error) {
	if strings.TrimSpace(req.Script) == "" {
		return nil, fmt.Errorf("probe: script is required")
	}

	stdout, stderr, err := p.run(ctx, req, "json")
	if err != nil {
		detail := strings.TrimSpace(stderr)
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("probe failed: %s", detail)
	}

	res := &ProbeResult{}
	if len(bytes.TrimSpace(stdout)) > 0 {
		if err := json.Unmarshal(stdout, &res.Metadata); err != nil {
			return nil, fmt.Errorf("probe: parse json output: %w", err)
		}
	}

	if opts.IncludeHuman {
		human, _, humanErr := p.run(ctx, req, "human")
		if humanErr == nil {
			res.Human = string(human)
		}
	}

	return res, nil
}

// run builds the run config for req, execs `stroppy probe` with the given output
// format, and returns stdout, stderr and the exec error. Mirrors the monolith's
// executeStroppyProbeFormat.
func (p *StroppyProber) run(ctx context.Context, req ProbeRequest, outputFormat string) ([]byte, string, error) {
	script := req.Script
	rc := &stroppypb.RunConfig{
		Version: "1",
		Script:  &script,
	}
	if req.SQL != "" {
		sqlArg := req.SQL
		rc.Sql = &sqlArg
	}

	if req.DriverType != "" {
		driverCfg := &stroppypb.DriverRunConfig{
			DriverType: req.DriverType,
			Url:        defaultDriverURL(req.DriverType),
		}
		if req.PoolSize > 0 {
			maxConns := int32(req.PoolSize)
			driverCfg.Pool = &stroppypb.DriverRunConfig_PoolConfig{
				MaxConns: &maxConns,
				MinConns: &maxConns,
			}
		}
		rc.Drivers = map[uint32]*stroppypb.DriverRunConfig{0: driverCfg}
	}

	for k, v := range req.Env {
		key := strings.ToUpper(strings.TrimSpace(k))
		if key == "" {
			continue
		}
		if rc.Env == nil {
			rc.Env = make(map[string]string)
		}
		rc.Env[key] = v
	}
	if req.ScaleFactor > 0 {
		if rc.Env == nil {
			rc.Env = make(map[string]string)
		}
		rc.Env["SCALE_FACTOR"] = strconv.FormatFloat(req.ScaleFactor, 'f', -1, 64)
	}
	if req.PoolSize > 0 {
		if rc.Env == nil {
			rc.Env = make(map[string]string)
		}
		rc.Env["POOL_SIZE"] = fmt.Sprintf("%d", req.PoolSize)
	}

	configBytes, err := protojson.MarshalOptions{UseProtoNames: false}.Marshal(rc)
	if err != nil {
		return nil, "", fmt.Errorf("marshal config: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "stroppy-probe-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	for _, f := range req.Files {
		name, err := safeProbeWorkloadFileName(f.Name)
		if err != nil {
			return nil, "", err
		}
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(f.Content), 0o644); err != nil {
			return nil, "", fmt.Errorf("write workload file %q: %w", name, err)
		}
	}

	configPath := filepath.Join(tmpDir, "stroppy-config.json")
	if err := os.WriteFile(configPath, configBytes, 0o644); err != nil {
		return nil, "", fmt.Errorf("write config: %w", err)
	}

	binPath, err := p.resolveBinary(ctx, req.Version)
	if err != nil {
		return nil, "", err
	}

	args := []string{"probe", "-f", configPath}
	if outputFormat != "" {
		args = append(args, "-o", outputFormat)
	}
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = tmpDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return stdout.Bytes(), stderr.String(), err
}

// resolveBinary returns a path to a usable stroppy executable, mirroring the
// monolith's resolveStroppyBinary but defaulting to the configured upstream
// instead of requiring a version.
//
// Precedence:
//  1. STROPPY_PROBE_BIN (explicit path);
//  2. an explicit per-request version (release tag or "commit:<sha>");
//  3. the configured upstream (Config.StroppyUpstream): a local path served
//     straight, or an http(s) tarball/raw-exe downloaded + cached once;
//  4. a "stroppy" on PATH.
func (p *StroppyProber) resolveBinary(ctx context.Context, version string) (string, error) {
	if override := strings.TrimSpace(os.Getenv("STROPPY_PROBE_BIN")); override != "" {
		return override, nil
	}

	version = strings.TrimSpace(version)

	p.mu.Lock()
	defer p.mu.Unlock()

	if version != "" {
		if sha, ok := strings.CutPrefix(version, "commit:"); ok {
			return p.ensureCommitBinary(ctx, sha)
		}
		return p.ensureReleaseBinary(ctx, version)
	}

	if p.upstream != "" {
		// A local path (no http scheme) is used straight from disk — the same
		// stand-in artifact the gateway serves from a baked path.
		if !strings.HasPrefix(p.upstream, "http://") && !strings.HasPrefix(p.upstream, "https://") {
			if fileExists(p.upstream) {
				return p.upstream, nil
			}
			return "", fmt.Errorf("stroppy upstream path %q not found", p.upstream)
		}
		return p.ensureUpstreamBinary(ctx)
	}

	if path, err := exec.LookPath("stroppy"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("no stroppy binary: set STROPPY_PROBE_BIN, configure STROPPY_UPSTREAM, supply a version, or install stroppy on PATH")
}

func (p *StroppyProber) binaryCacheDir() string {
	if p.cacheDir != "" {
		return p.cacheDir
	}
	return filepath.Join(os.TempDir(), "stroppy-cloud", "stroppy-binaries")
}

// ensureUpstreamBinary downloads + caches the configured upstream artifact. The
// upstream may be a .tar.gz (extract the "stroppy" entry) or a raw executable.
func (p *StroppyProber) ensureUpstreamBinary(ctx context.Context) (string, error) {
	dir := filepath.Join(p.binaryCacheDir(), "upstream-"+safeCacheName(p.upstream))
	binPath := filepath.Join(dir, "stroppy")
	if fileExists(binPath) {
		return binPath, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create stroppy cache dir: %w", err)
	}
	if strings.HasSuffix(p.upstream, ".tar.gz") || strings.HasSuffix(p.upstream, ".tgz") {
		if err := downloadStroppyTarball(ctx, p.upstream, binPath); err != nil {
			return "", err
		}
		return binPath, nil
	}
	if err := downloadRawExecutable(ctx, p.upstream, binPath); err != nil {
		return "", err
	}
	return binPath, nil
}

func (p *StroppyProber) ensureReleaseBinary(ctx context.Context, version string) (string, error) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if version == "" {
		return "", fmt.Errorf("stroppy release version is empty")
	}
	dir := filepath.Join(p.binaryCacheDir(), safeCacheName("v"+version))
	binPath := filepath.Join(dir, "stroppy")
	if fileExists(binPath) {
		return binPath, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create stroppy cache dir: %w", err)
	}
	url := fmt.Sprintf("https://github.com/stroppy-io/stroppy/releases/download/v%s/stroppy_linux_amd64.tar.gz", version)
	if err := downloadStroppyTarball(ctx, url, binPath); err != nil {
		return "", err
	}
	return binPath, nil
}

func (p *StroppyProber) ensureCommitBinary(ctx context.Context, sha string) (string, error) {
	sha = strings.ToLower(strings.TrimSpace(sha))
	if len(sha) > 7 {
		sha = sha[:7]
	}
	if len(sha) < 7 {
		return "", fmt.Errorf("commit-pinned stroppy version needs at least 7 hex characters")
	}
	for _, r := range sha {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return "", fmt.Errorf("invalid commit sha %q", sha)
		}
	}
	dir := filepath.Join(p.binaryCacheDir(), "commit-"+sha)
	binPath := filepath.Join(dir, "stroppy")
	if fileExists(binPath) {
		return binPath, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create stroppy cache dir: %w", err)
	}
	url := fmt.Sprintf("https://github.com/stroppy-io/stroppy/releases/download/nightly-%s/stroppy", sha)
	if err := downloadRawExecutable(ctx, url, binPath); err != nil {
		return "", err
	}
	return binPath, nil
}

func downloadStroppyTarball(ctx context.Context, url, dest string) error {
	resp, err := httpGet(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("read stroppy tarball: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	tmp := dest + ".tmp"
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read stroppy tarball: %w", err)
		}
		if h.FileInfo().IsDir() || filepath.Base(h.Name) != "stroppy" {
			continue
		}
		if err := writeExecutable(tmp, tr); err != nil {
			return err
		}
		return os.Rename(tmp, dest)
	}
	return fmt.Errorf("stroppy binary not found in release tarball")
}

func downloadRawExecutable(ctx context.Context, url, dest string) error {
	resp, err := httpGet(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	tmp := dest + ".tmp"
	if err := writeExecutable(tmp, resp.Body); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

// proberHTTPClient mirrors the monolith's bounded-timeout client: GitHub's
// release-assets CDN occasionally hangs on TLS handshake / first byte, which
// surfaces as a probe failure; explicit timeouts + a few retries keep the path
// reliable.
var proberHTTPClient = &http.Client{
	Timeout: 5 * time.Minute,
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	},
}

func httpGet(ctx context.Context, url string) (*http.Response, error) {
	const attempts = 3
	var lastErr error
	for i := 0; i < attempts; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := proberHTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("download stroppy binary: %w", err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(i+1) * 2 * time.Second):
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			if resp.StatusCode >= 500 && i < attempts-1 {
				lastErr = fmt.Errorf("download stroppy binary %d: %s", resp.StatusCode, string(body))
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Duration(i+1) * 2 * time.Second):
				}
				continue
			}
			return nil, fmt.Errorf("download stroppy binary %d: %s", resp.StatusCode, string(body))
		}
		return resp, nil
	}
	return nil, lastErr
}

func writeExecutable(path string, src io.Reader) error {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("create stroppy binary: %w", err)
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return fmt.Errorf("write stroppy binary: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close stroppy binary: %w", err)
	}
	return os.Chmod(path, 0o755)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func safeCacheName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func safeProbeWorkloadFileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("workload file name is required")
	}
	clean := filepath.Clean(name)
	if clean != name || filepath.Base(name) != name || strings.Contains(name, "\x00") {
		return "", fmt.Errorf("invalid workload file name %q", name)
	}
	return name, nil
}

// defaultDriverURL returns a stand-in connection URL for a driver type so the
// probe can construct a driver config without a real database (probe never
// connects). Mirrors the monolith's defaultDriverURL.
func defaultDriverURL(driverType string) string {
	switch driverType {
	case "postgres":
		return "postgres://postgres:postgres@localhost:5432"
	case "mysql":
		return "root@tcp(localhost:3306)/"
	case "picodata":
		return "postgres://admin:T0psecret@localhost:1331"
	case "ydb":
		return "grpc://localhost:2136/Root/testdb"
	default:
		return "localhost"
	}
}
