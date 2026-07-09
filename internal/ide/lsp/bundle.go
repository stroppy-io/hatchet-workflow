// Package lsp implements the stroppy-yaml language server: a thin shell
// over internal/services/dsl.DslService (Check/ComposedSchema/Preview) that
// speaks the Language Server Protocol so an editor (code-server's VS Code
// frontend, via the extension in internal/ide/lsp/extension) gets live
// diagnostics, completion, and a preview command for cluster.yaml/
// workflow.yaml bundles. No validation, schema derivation, or compilation
// logic lives here — every domain decision is delegated to DslService,
// exactly the compiler this codebase already ships (see diagnostics.go,
// completion.go, preview.go's doc comments for the specific delegation).
package lsp

import (
	"os"
	"path/filepath"
)

// bundleMarkers are the two fixed-layout files internal/services/dsl.
// service.go's clusterFile/workflowFile constants name — a directory
// containing both is a compilable bundle root.
const (
	clusterFile  = "cluster.yaml"
	workflowFile = "workflow.yaml"
)

// FindBundleRoot walks up from path's directory looking for a directory
// containing both "cluster.yaml" and "workflow.yaml" — the same fixed-layout
// convention internal/services/dsl.service.go's clusterFile/workflowFile
// constants encode. Returns "" if no such ancestor exists at or under root
// (root bounds the walk so a workspace with no bundle at all terminates,
// and — load-bearing for multi-tenancy, see the package's isolation note in
// server.go — the walk never climbs ABOVE root, so a caller wiring root to
// the LSP's own workspace folder cannot have FindBundleRoot wander into a
// parent directory that folder does not actually contain).
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
		// Never climb above root: once dir is no longer inside root (or we hit
		// the filesystem root without ever reaching `root`), stop.
		if parent == dir || !isWithin(root, parent) {
			return ""
		}
		dir = parent
	}
}

// isWithin reports whether path is root itself or a descendant of it.
func isWithin(root, path string) bool {
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !hasDotDotPrefix(rel)
}

func hasDotDotPrefix(rel string) bool {
	return len(rel) >= 2 && rel[0] == '.' && rel[1] == '.' &&
		(len(rel) == 2 || rel[2] == filepath.Separator)
}

func hasBundleMarkers(dir string) bool {
	_, cErr := os.Stat(filepath.Join(dir, clusterFile))
	_, wErr := os.Stat(filepath.Join(dir, workflowFile))
	return cErr == nil && wErr == nil
}

// SnapshotFiles reads every regular file under bundleRoot into the
// files map[string][]byte shape CompileBundle/Check/ComposedSchema/Preview
// all expect (internal/services/dsl.DslService's CheckRequest/
// ComposedSchemaRequest/PreviewRequest.Files), keyed by their path relative
// to bundleRoot (slash-separated, matching internal/dsl/include.Sources'
// convention). Only ever walks INSIDE bundleRoot — filepath.WalkDir has no
// way to escape the root it is given, so this cannot read a file outside
// the caller-supplied bundle directory regardless of what bundleRoot's
// caller passes as `path` elsewhere.
func SnapshotFiles(bundleRoot string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(bundleRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip VCS/editor metadata directories a real worktree will have
			// (.git, .vscode) — CompileBundle would otherwise choke on
			// non-bundle content Check/ComposedSchema/Preview never expect to
			// see in a "files" map.
			if d.Name() == ".git" || d.Name() == ".vscode" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(bundleRoot, p)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(p) //nolint:gosec // bounded to bundleRoot by WalkDir above.
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
