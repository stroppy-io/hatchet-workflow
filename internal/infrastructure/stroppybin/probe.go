package stroppybin

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
)

type ProbeInput struct {
	Version      string
	Script       string
	SQL          string
	DriverType   string
	PoolSize     uint32
	ScaleFactor  uint32
	Env          map[string]string
	Files        []WorkloadFile
	IncludeHuman bool
}

type WorkloadFile struct {
	Name    string
	Content string
}

type ProbeResult struct {
	StdoutJSON []byte
	Human      string
	HumanErr   string
}

func (r *Runner) RunProbe(ctx context.Context, in ProbeInput) (*ProbeResult, error) {
	tmp, err := os.MkdirTemp("", "stroppy-probe-*")
	if err != nil {
		return nil, fmt.Errorf("stroppybin: mkdir tmp: %w", err)
	}
	defer os.RemoveAll(tmp)

	cfg := buildRunConfig(in)
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("stroppybin: marshal cfg: %w", err)
	}
	cfgPath := filepath.Join(tmp, "stroppy-config.json")
	if err := os.WriteFile(cfgPath, cfgBytes, 0o600); err != nil {
		return nil, fmt.Errorf("stroppybin: write cfg: %w", err)
	}
	for _, f := range in.Files {
		safe := filepath.Base(f.Name)
		if safe != f.Name || safe == "" || safe == "." || safe == ".." {
			return nil, fmt.Errorf("stroppybin: unsafe workload file name %q", f.Name)
		}
		if err := os.WriteFile(filepath.Join(tmp, safe), []byte(f.Content), 0o600); err != nil {
			return nil, fmt.Errorf("stroppybin: write workload file: %w", err)
		}
	}

	stdout, stderr, err := r.Exec(ctx, in.Version, ExecOptions{
		Args: []string{"probe", "-f", cfgPath, "-o", "json"},
		Dir:  tmp,
	})
	if err != nil {
		return nil, fmt.Errorf("stroppybin: probe failed: %w (stderr=%s)", err, string(stderr))
	}
	res := &ProbeResult{StdoutJSON: stdout}
	if in.IncludeHuman {
		hStdout, hStderr, hErr := r.Exec(ctx, in.Version, ExecOptions{
			Args: []string{"probe", "-f", cfgPath, "-o", "human"},
			Dir:  tmp,
		})
		if hErr != nil {
			res.HumanErr = string(hStderr)
		} else {
			res.Human = string(hStdout)
		}
	}
	return res, nil
}

func buildRunConfig(in ProbeInput) map[string]any {
	rc := map[string]any{
		"version": "1",
		"script":  in.Script,
	}
	if in.SQL != "" {
		rc["sql"] = in.SQL
	}
	if in.DriverType != "" {
		drv := map[string]any{"driver_type": in.DriverType, "url": defaultDriverURL(in.DriverType)}
		if in.PoolSize > 0 {
			drv["pool"] = map[string]any{"max_conns": in.PoolSize, "min_conns": in.PoolSize}
		}
		rc["drivers"] = map[string]any{"0": drv}
	}
	env := map[string]string{}
	maps.Copy(env, in.Env)
	if in.ScaleFactor > 0 {
		env["SCALE_FACTOR"] = fmt.Sprintf("%d", in.ScaleFactor)
	}
	if in.PoolSize > 0 {
		env["POOL_SIZE"] = fmt.Sprintf("%d", in.PoolSize)
	}
	if len(env) > 0 {
		rc["env"] = env
	}
	return rc
}

func defaultDriverURL(driver string) string {
	switch driver {
	case "postgres":
		return "postgres://stroppy:stroppy@127.0.0.1:5432/stroppy?sslmode=disable"
	case "mysql":
		return "mysql://stroppy:stroppy@127.0.0.1:3306/stroppy"
	case "picodata":
		return "picodata://admin:admin@127.0.0.1:3301/"
	default:
		return ""
	}
}
