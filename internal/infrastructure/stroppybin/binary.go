package stroppybin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type Runner struct {
	defaultVersion string
	binariesDir    string

	mu       sync.Mutex
	resolved map[string]string
}

func New(defaultVersion, binariesDir string) *Runner {
	return &Runner{
		defaultVersion: defaultVersion,
		binariesDir:    binariesDir,
		resolved:       make(map[string]string),
	}
}

func (r *Runner) Resolve(version string) (string, error) {
	if version == "" {
		version = r.defaultVersion
	}
	if override := strings.TrimSpace(os.Getenv("STROPPY_PROBE_BIN")); override != "" {
		return override, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if cached, ok := r.resolved[version]; ok {
		return cached, nil
	}
	candidates := []string{
		filepath.Join(r.binariesDir, "stroppy-"+version),
		filepath.Join(r.binariesDir, version, "stroppy"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			r.resolved[version] = c
			return c, nil
		}
	}
	return "", fmt.Errorf("stroppybin: binary not found for version %q (looked in %v)", version, candidates)
}

type ExecOptions struct {
	Args  []string
	Stdin []byte
	Dir   string
	Env   []string
}

func (r *Runner) Exec(ctx context.Context, version string, opts ExecOptions) ([]byte, []byte, error) {
	bin, err := r.Resolve(version)
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.CommandContext(ctx, bin, opts.Args...)
	cmd.Dir = opts.Dir
	cmd.Env = append(os.Environ(), opts.Env...)
	if len(opts.Stdin) > 0 {
		cmd.Stdin = strings.NewReader(string(opts.Stdin))
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return []byte(stdout.String()), []byte(stderr.String()), err
}
