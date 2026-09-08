package activities

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/graphene-ci/pipeline/pkg/machine"
)

// WriteFiles writes the files under Dir on the machine. A relative Dir is
// resolved against the run workspace, which is the same absolute path on
// the machine, in the agent container and for the machine's docker daemon —
// so the paths returned are valid bind-mount sources. Idempotent: rewrites
// in place.
func WriteFiles(_ context.Context, req WriteFilesRequest) (WriteFilesResult, error) {
	dir, err := workspaceDir(req.Dir)
	if err != nil {
		return WriteFilesResult{}, fmt.Errorf("write files: %w", err)
	}
	var out WriteFilesResult
	for _, f := range req.Files {
		rel := strings.TrimLeft(f.Path, "/")
		if rel == "" || strings.Contains(rel, "..") {
			return WriteFilesResult{}, fmt.Errorf("write files: bad path %q", f.Path)
		}
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return WriteFilesResult{}, err
		}
		mode := os.FileMode(0o644)
		if f.Mode != "" {
			m, err := strconv.ParseUint(f.Mode, 8, 32)
			if err != nil {
				return WriteFilesResult{}, fmt.Errorf("write files: mode %q: %w", f.Mode, err)
			}
			mode = os.FileMode(m)
		}
		if err := os.WriteFile(abs, []byte(f.Content), mode); err != nil {
			return WriteFilesResult{}, err
		}
		if err := os.Chmod(abs, mode); err != nil {
			return WriteFilesResult{}, err
		}
		out.Paths = append(out.Paths, abs)
	}
	return out, nil
}

// workspaceDir resolves a directory of the run workspace: absolute paths
// pass through, relative ones are joined to the agent's workspace root.
func workspaceDir(dir string) (string, error) {
	if dir == "" || strings.Contains(dir, "..") {
		return "", fmt.Errorf("bad dir %q", dir)
	}
	if filepath.IsAbs(dir) {
		return dir, nil
	}
	ws := machine.Workspace()
	if ws == "" {
		return "", fmt.Errorf("relative dir %q but no %s — not running in an agent container", dir, machine.EnvWorkspace)
	}
	return filepath.Join(ws, dir), nil
}
