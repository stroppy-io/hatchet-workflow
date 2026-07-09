package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindBundleRoot_WalksUpToClusterAndWorkflow(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "workflows", "tpcc")
	if err := os.MkdirAll(filepath.Join(bundle, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "cluster.yaml"), []byte("provider:\n  use: docker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "workflow.yaml"), []byte("name: tpcc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	openFile := filepath.Join(bundle, "components", "pg.yaml")
	if err := os.WriteFile(openFile, []byte("name: pg\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := FindBundleRoot(root, openFile)
	if got != bundle {
		t.Fatalf("bundle root = %q, want %q", got, bundle)
	}
}

func TestFindBundleRoot_NoAncestorReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	loose := filepath.Join(root, "notes.md")
	if err := os.WriteFile(loose, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FindBundleRoot(root, loose); got != "" {
		t.Fatalf("bundle root = %q, want empty", got)
	}
}

// TestFindBundleRoot_NeverClimbsAboveRoot is the multi-tenancy-relevant
// case: a bundle sitting in the PARENT of root (e.g. another scope's
// worktree, if two scopes' directories happened to be siblings) must never
// be found by a search rooted at a narrower directory. This is the
// property server.go's wiring relies on: root is always the LSP session's
// own workspace folder (one scope's worktree, per internal/ide.Manager's
// per-scope mount — see manager.go), so FindBundleRoot must never surface
// a bundle living outside it.
func TestFindBundleRoot_NeverClimbsAboveRoot(t *testing.T) {
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, "cluster.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "workflow.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	scopeRoot := filepath.Join(parent, "scope-a")
	if err := os.MkdirAll(filepath.Join(scopeRoot, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	openFile := filepath.Join(scopeRoot, "components", "pg.yaml")
	if err := os.WriteFile(openFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := FindBundleRoot(scopeRoot, openFile); got != "" {
		t.Fatalf("bundle root = %q, want empty (must not climb above scopeRoot into parent)", got)
	}
}

func TestSnapshotFiles_ReadsAllFilesRelativeToRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cluster.yaml"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "components", "pg.yaml"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A .git directory must be skipped — it is never part of a
	// CompileBundle-shaped files map.
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := SnapshotFiles(root)
	if err != nil {
		t.Fatalf("SnapshotFiles: %v", err)
	}
	if string(files["cluster.yaml"]) != "a" {
		t.Fatalf("cluster.yaml = %q, want %q", files["cluster.yaml"], "a")
	}
	if string(files["components/pg.yaml"]) != "b" {
		t.Fatalf("components/pg.yaml = %q, want %q", files["components/pg.yaml"], "b")
	}
	if _, ok := files[".git/HEAD"]; ok {
		t.Fatal("SnapshotFiles must not include .git contents")
	}
}
