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

func TestFindBundleRoot_OpenFileIsClusterYamlItself(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "bundle")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "cluster.yaml"), []byte("provider:\n  use: docker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "workflow.yaml"), []byte("name: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := FindBundleRoot(root, filepath.Join(bundle, "cluster.yaml"))
	if got != bundle {
		t.Fatalf("bundle root = %q, want %q", got, bundle)
	}
}

func TestFindBundleRoot_StopsAtRootBoundary(t *testing.T) {
	// A root that itself has no bundle markers, and no ancestor above it
	// should ever be consulted (the walk must not escape root upward).
	outer := t.TempDir()
	if err := os.WriteFile(filepath.Join(outer, "cluster.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outer, "workflow.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(outer, "workspace")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	loose := filepath.Join(root, "notes.md")
	if err := os.WriteFile(loose, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FindBundleRoot(root, loose); got != "" {
		t.Fatalf("bundle root = %q, want empty (must not climb above root into %q)", got, outer)
	}
}

func TestSnapshotFiles_ReadsEveryFileRelativeToRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "providers", "docker"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cluster.yaml"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflow.yaml"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "providers", "docker", "manifest.yaml"), []byte("c"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := SnapshotFiles(root)
	if err != nil {
		t.Fatalf("SnapshotFiles: %v", err)
	}
	want := map[string]string{
		"cluster.yaml":                   "a",
		"workflow.yaml":                  "b",
		"providers/docker/manifest.yaml": "c",
	}
	if len(files) != len(want) {
		t.Fatalf("got %d files, want %d: %+v", len(files), len(want), files)
	}
	for path, content := range want {
		got, ok := files[path]
		if !ok {
			t.Fatalf("missing file %q in snapshot: %+v", path, files)
		}
		if string(got) != content {
			t.Fatalf("file %q = %q, want %q", path, got, content)
		}
	}
}
