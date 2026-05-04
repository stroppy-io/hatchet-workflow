package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"

	stroppypb "github.com/stroppy-io/stroppy/pkg/common/proto/stroppy"
)

// probeRequest is the JSON body for POST /api/v1/probe.
type probeRequest struct {
	Version     string               `json:"version,omitempty"`
	Script      string               `json:"script"`                // e.g. "tpcc/procs", "tpcb/tx"
	SQL         string               `json:"sql,omitempty"`         // optional second SQL argument
	DriverType  string               `json:"driver_type,omitempty"` // e.g. "postgres", "mysql", "picodata"
	PoolSize    int                  `json:"pool_size,omitempty"`
	ScaleFactor int                  `json:"scale_factor,omitempty"`
	Env         map[string]string    `json:"env,omitempty"`
	Files       []types.WorkloadFile `json:"files,omitempty"`
}

// stroppyProbe handles POST /api/v1/probe.
// It builds a minimal stroppy-config.json, runs `stroppy probe -f <file> -o json`,
// and returns the probe output (env declarations, steps, sql sections, driver setups).
func (s *Server) stroppyProbe(w http.ResponseWriter, r *http.Request) {
	var req probeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if req.Script == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "script is required"})
		return
	}

	stdout, stderr, err := s.executeStroppyProbe(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error":  "probe failed: " + err.Error(),
			"output": stderr,
		})
		return
	}

	// Return probe JSON output directly (stdout only, no log noise).
	w.Header().Set("Content-Type", "application/json")
	w.Write(stdout)
}

func (s *Server) executeStroppyProbe(ctx context.Context, req probeRequest) ([]byte, string, error) {
	script := req.Script
	rc := &stroppypb.RunConfig{
		Version: "1",
		Script:  &script,
	}
	if req.SQL != "" {
		sqlArg := req.SQL
		rc.Sql = &sqlArg
	}

	// Add driver if specified.
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

	// Add env overrides.
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
		rc.Env["SCALE_FACTOR"] = fmt.Sprintf("%d", req.ScaleFactor)
	}
	if req.PoolSize > 0 {
		if rc.Env == nil {
			rc.Env = make(map[string]string)
		}
		rc.Env["POOL_SIZE"] = fmt.Sprintf("%d", req.PoolSize)
	}

	// Serialize config to JSON.
	configBytes, err := protojson.MarshalOptions{
		UseProtoNames: false,
	}.Marshal(rc)
	if err != nil {
		return nil, "", fmt.Errorf("marshal config: %w", err)
	}

	// Write temp file.
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
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(f.Content), 0644); err != nil {
			return nil, "", fmt.Errorf("write workload file %q: %w", name, err)
		}
	}

	configPath := filepath.Join(tmpDir, "stroppy-config.json")
	if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
		return nil, "", fmt.Errorf("write config: %w", err)
	}

	binPath, err := s.resolveStroppyBinary(ctx, req.Version)
	if err != nil {
		return nil, "", err
	}

	// Run stroppy probe — capture stdout (JSON) separately from stderr (logs).
	cmd := exec.CommandContext(ctx, binPath, "probe", "-f", configPath, "-o", "json")
	cmd.Dir = tmpDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return stdout.Bytes(), stderr.String(), err
	}

	return stdout.Bytes(), stderr.String(), nil
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

func (s *Server) probeRunWorkload(ctx context.Context, cfg types.RunConfig) error {
	// Raw overrides are validated by protojson parsing in the run task. They
	// may intentionally diverge from the field-level wizard state, so probing
	// those fields here would create false negatives for advanced users.
	if cfg.Stroppy.ConfigOverrideJSON != "" {
		var rc stroppypb.RunConfig
		if err := protojson.Unmarshal([]byte(cfg.Stroppy.ConfigOverrideJSON), &rc); err != nil {
			return fmt.Errorf("invalid stroppy config override: %w", err)
		}
		return nil
	}

	script := cfg.Stroppy.Script
	if script == "" {
		script = cfg.Stroppy.Workload
	}
	if script == "" {
		script = "tpcc/procs"
	}

	protocol := cfg.Stroppy.Protocol
	if protocol == "" {
		protocol = types.DefaultProtocol(cfg.Database.Kind)
	}
	driverType := string(cfg.Database.Kind)
	if meta, ok := types.Protocols[protocol]; ok && meta.DriverType != "" {
		driverType = meta.DriverType
	}

	_, stderr, err := s.executeStroppyProbe(ctx, probeRequest{
		Version:     cfg.Stroppy.Version,
		Script:      script,
		SQL:         cfg.Stroppy.SQL,
		DriverType:  driverType,
		PoolSize:    cfg.Stroppy.PoolSize,
		ScaleFactor: cfg.Stroppy.ScaleFactor,
		Env:         cfg.Stroppy.Env,
		Files:       cfg.Stroppy.Files,
	})
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

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
