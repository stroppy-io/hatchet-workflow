package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/uri"
)

func mustWriteBundle(t *testing.T, root string) string {
	t.Helper()
	bundle := filepath.Join(root, "workflows", "tpcc")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "cluster.yaml"), []byte("version: 1\n"+
		"provider:\n  use: docker\n"+
		"machines:\n  db:\n    count: 1\n    resources: { cpu: 2, ram: 2g, disk: { size: 10g, type: ssd } }\n"+
		"services:\n  postgres:\n    on: db\n    image: postgres:17\n    network: host\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "workflow.yaml"), []byte("jobs:\n  postgres:\n    service: postgres\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return bundle
}

func TestSession_ResolvePath_AcceptsPathWithinRoot(t *testing.T) {
	root := t.TempDir()
	bundle := mustWriteBundle(t, root)
	sess := NewSession(root)

	clusterPath := filepath.Join(bundle, "cluster.yaml")
	got, err := sess.ResolvePath(string(uri.File(clusterPath)))
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	// Compare via EvalSymlinks-insensitive filepath.Clean equality (macOS
	// temp dirs can have a /private prefix quirk; not relevant on Linux CI
	// but Clean-compare rather than raw string-compare defensively).
	if filepath.Clean(got) != filepath.Clean(clusterPath) {
		t.Fatalf("ResolvePath = %q, want %q", got, clusterPath)
	}
}

// TestSession_ResolvePath_RejectsPathOutsideWorkspaceRoot is the isolation
// proof the task's hard rules require: "an LSP session must only ever see
// the worktree of the scope it was authorized for ... make sure the LSP
// opens no side door." A session's workspaceRoot is set once at
// `initialize` to the scope's own worktree (e.g.
// /var/lib/stroppy-ide/org:<tenant-id>, per internal/ide.Manager's
// volume-subpath mount — see spc-t4-report.md). Since code-server itself
// only ever mounts that one subtree into the container, a request naming a
// file:// URI outside it can only arise from a hostile/buggy client — this
// proves the LSP process refuses to read it rather than silently trusting
// whatever path a request carries.
func TestSession_ResolvePath_RejectsPathOutsideWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	mustWriteBundle(t, root)
	sess := NewSession(root)

	outside := filepath.Join(filepath.Dir(root), "other-scope-worktree", "cluster.yaml")
	if _, err := sess.ResolvePath(string(uri.File(outside))); err == nil {
		t.Fatalf("ResolvePath(%q) succeeded, want a rejection (path escapes workspace root %q)", outside, root)
	}
}

func TestSession_ResolvePath_RejectsPathTraversalWithinURI(t *testing.T) {
	root := t.TempDir()
	mustWriteBundle(t, root)
	sess := NewSession(root)

	// Even a request whose raw path STARTS under root but climbs out via
	// ".." must be rejected after Clean — the same class of bug T1-T3's
	// path-traversal fix and this task's own CheckRejectsPathTraversalInProviderModule
	// test guard against elsewhere in this codebase.
	traversal := filepath.Join(root, "workflows", "..", "..", "etc", "passwd")
	if _, err := sess.ResolvePath(string(uri.File(traversal))); err == nil {
		t.Fatalf("ResolvePath(%q) succeeded, want a rejection", traversal)
	}
}

func TestSession_BundleFiles_OverlayWinsOverDiskForOpenDocument(t *testing.T) {
	root := t.TempDir()
	bundle := mustWriteBundle(t, root)
	sess := NewSession(root)

	clusterPath := filepath.Join(bundle, "cluster.yaml")
	overlayContent := []byte("version: 1\nprovider:\n  use: docker\n# edited-in-buffer, not yet saved\n" +
		"machines:\n  db:\n    count: 1\n    resources: { cpu: 2, ram: 2g, disk: { size: 10g, type: ssd } }\n" +
		"services:\n  postgres:\n    on: db\n    image: postgres:17\n    network: host\n")
	sess.SetOverlay(clusterPath, overlayContent)

	bundleRoot, files, err := sess.BundleFiles(clusterPath)
	if err != nil {
		t.Fatalf("BundleFiles: %v", err)
	}
	if bundleRoot != bundle {
		t.Fatalf("bundle root = %q, want %q", bundleRoot, bundle)
	}
	got, ok := files["cluster.yaml"]
	if !ok {
		t.Fatal("expected cluster.yaml in the snapshot")
	}
	if string(got) != string(overlayContent) {
		t.Fatalf("BundleFiles returned on-disk content, want the open buffer's unsaved overlay content:\ngot:  %s\nwant: %s", got, overlayContent)
	}
}

func TestSession_BundleFiles_NoOverlayFallsBackToDisk(t *testing.T) {
	root := t.TempDir()
	bundle := mustWriteBundle(t, root)
	sess := NewSession(root)

	_, files, err := sess.BundleFiles(filepath.Join(bundle, "cluster.yaml"))
	if err != nil {
		t.Fatalf("BundleFiles: %v", err)
	}
	if _, ok := files["workflow.yaml"]; !ok {
		t.Fatalf("expected workflow.yaml present from disk, got %+v", files)
	}
}

func TestSession_RemoveOverlay_FallsBackToDiskAfterClose(t *testing.T) {
	root := t.TempDir()
	bundle := mustWriteBundle(t, root)
	sess := NewSession(root)
	clusterPath := filepath.Join(bundle, "cluster.yaml")

	sess.SetOverlay(clusterPath, []byte("# buffer only\n"))
	sess.RemoveOverlay(clusterPath)

	_, files, err := sess.BundleFiles(clusterPath)
	if err != nil {
		t.Fatalf("BundleFiles: %v", err)
	}
	if string(files["cluster.yaml"]) == "# buffer only\n" {
		t.Fatal("expected the overlay to be gone after RemoveOverlay, still saw buffer content")
	}
}
