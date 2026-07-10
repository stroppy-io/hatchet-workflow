// Package lsp is a thin shell over internal/services/dsl's compiler surface
// (DslService.Check/ComposedSchema/Preview) for the stroppy-yaml language
// server: bundle discovery on disk, diagnostics, completion, and a preview
// command. It deliberately contains NO validation/schema/compile logic of
// its own — every domain answer is delegated to internal/services/dsl and
// internal/dsl/schema, which already implement it and are exercised by
// their own extensive test suites. See this package's doc in
// .superpowers/sdd/spc-t5-t7-report.md for the full delegation map.
package lsp

import (
	"os"
	"path/filepath"
)

// clusterFileName/workflowFileName mirror internal/services/dsl/service.go's
// unexported clusterFile/workflowFile constants ("cluster.yaml"/
// "workflow.yaml") — the fixed bundle-root markers that package intentionally
// keeps unexported (it's a pure compiler package, no filesystem concern of
// its own). Duplicated here as the two literal strings rather than importing
// them, since internal/services/dsl exports no constant for this; if that
// package ever exports its own, switch to it instead of drifting further.
const (
	clusterFileName  = "cluster.yaml"
	workflowFileName = "workflow.yaml"
)

// FindBundleRoot walks up from path's directory looking for the nearest
// ancestor directory (bounded by root) that contains both cluster.yaml and
// workflow.yaml — the same fixed bundle-root convention
// internal/services/dsl/service.go's clusterFile/workflowFile constants
// encode. Returns "" if no such ancestor exists at or below root (the walk
// never climbs above root, so a workspace with no bundle at all terminates
// instead of wandering into unrelated parent directories on disk).
func FindBundleRoot(root, path string) string {
	root = filepath.Clean(root)
	dir := filepath.Clean(filepath.Dir(path))

	for {
		if hasBundleMarkers(dir) {
			return dir
		}
		if dir == root {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root without ever reaching root itself
			// (path was not under root) — stop rather than loop forever.
			return ""
		}
		dir = parent
	}
}

func hasBundleMarkers(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, clusterFileName)); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, workflowFileName)); err != nil {
		return false
	}
	return true
}

// SnapshotFiles reads every regular file under bundleRoot into the
// files map[string][]byte shape DslService.Check/ComposedSchema/Preview all
// expect (internal/proto/cloud/v1/dsl.CheckRequest.Files etc.), keyed by
// its path relative to bundleRoot, slash-separated — matching
// internal/services/dsl/service_test.go's loadBundle helper exactly (same
// convention the RPC handlers' own tests already load fixtures with), so a
// snapshot taken here round-trips identically through Check/ComposedSchema/
// Preview.
func SnapshotFiles(bundleRoot string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.Walk(bundleRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(bundleRoot, p)
		if relErr != nil {
			return relErr
		}
		content, readErr := os.ReadFile(p) //nolint:gosec // bundleRoot is a caller-controlled worktree path, not user input.
		if readErr != nil {
			return readErr
		}
		files[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
