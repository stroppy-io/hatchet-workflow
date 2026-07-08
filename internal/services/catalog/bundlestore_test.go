package catalog

import (
	"context"
	"testing"
)

func TestMemoryBundleStore_WriteReadFork(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryBundleStore()

	ref, err := store.Write(ctx, "", map[string][]byte{"manifest.yaml": []byte("name: yandex\n")})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if ref == "" {
		t.Fatal("write returned empty ref")
	}

	files, err := store.Read(ctx, ref)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(files["manifest.yaml"]) != "name: yandex\n" {
		t.Fatalf("read mismatch: %q", files["manifest.yaml"])
	}

	forked, err := store.Fork(ctx, ref)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if forked == ref {
		t.Fatal("fork must return a new ref, not the source ref")
	}
	forkedFiles, err := store.Read(ctx, forked)
	if err != nil {
		t.Fatalf("read forked: %v", err)
	}
	forkedFiles["manifest.yaml"] = []byte("mutated")
	origFiles, _ := store.Read(ctx, ref)
	if string(origFiles["manifest.yaml"]) != "name: yandex\n" {
		t.Fatal("mutating a Read() result must not affect the store (fork must deep-copy)")
	}
}

func TestMemoryBundleStore_ReadUnknownRef(t *testing.T) {
	_, err := NewMemoryBundleStore().Read(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error for unknown ref")
	}
}

func TestMemoryBundleStore_WriteDeepCopiesInput(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryBundleStore()

	input := map[string][]byte{"manifest.yaml": []byte("name: yandex\n")}
	ref, err := store.Write(ctx, "", input)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// Mutate the caller's map after Write; the store must be unaffected.
	input["manifest.yaml"][0] = 'X'

	files, err := store.Read(ctx, ref)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(files["manifest.yaml"]) != "name: yandex\n" {
		t.Fatalf("mutating the input after Write must not affect the store, got %q", files["manifest.yaml"])
	}
}

func TestMemoryBundleStore_ForkIndependentOfSourceEdits(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryBundleStore()

	ref, err := store.Write(ctx, "", map[string][]byte{"manifest.yaml": []byte("name: yandex\n")})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	forked, err := store.Fork(ctx, ref)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}

	// Overwrite the source ref's content via a fresh Write to a distinct ref
	// (refs are content-addressed, so "editing" ref means writing new content
	// under a new ref) and confirm the fork's own stored bytes are untouched.
	if _, err := store.Write(ctx, "", map[string][]byte{"manifest.yaml": []byte("name: mutated\n")}); err != nil {
		t.Fatalf("write: %v", err)
	}

	forkedFiles, err := store.Read(ctx, forked)
	if err != nil {
		t.Fatalf("read forked: %v", err)
	}
	if string(forkedFiles["manifest.yaml"]) != "name: yandex\n" {
		t.Fatalf("fork contents changed after unrelated write: %q", forkedFiles["manifest.yaml"])
	}
}

func TestFSBundleStore_WriteReadFork(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFSBundleStore(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx := context.Background()

	ref, err := store.Write(ctx, "", map[string][]byte{"manifest.yaml": []byte("name: docker\n")})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	files, err := store.Read(ctx, ref)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(files["manifest.yaml"]) != "name: docker\n" {
		t.Fatalf("mismatch: %q", files["manifest.yaml"])
	}
	forked, err := store.Fork(ctx, ref)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if forked == ref {
		t.Fatal("fork must produce a distinct ref")
	}
}

func TestFSBundleStore_ReadUnknownRef(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFSBundleStore(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := store.Read(context.Background(), "nope"); err == nil {
		t.Fatal("expected error for unknown ref")
	}
}

func TestFSBundleStore_NestedPaths(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFSBundleStore(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx := context.Background()

	files := map[string][]byte{
		"providers/yandex/manifest.yaml":  []byte("name: yandex\n"),
		"providers/yandex/module/main.tf": []byte("resource \"x\" {}\n"),
		"cluster.yaml":                    []byte("version: 1\n"),
	}
	ref, err := store.Write(ctx, "", files)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := store.Read(ctx, ref)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != len(files) {
		t.Fatalf("read %d files, want %d: %v", len(got), len(files), got)
	}
	for name, want := range files {
		if string(got[name]) != string(want) {
			t.Fatalf("file %q = %q, want %q", name, got[name], want)
		}
	}
}

func TestFSBundleStore_WriteRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFSBundleStore(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx := context.Background()

	_, err = store.Write(ctx, "", map[string][]byte{
		"../../../../etc/cron.d/evil": []byte("payload\n"),
	})
	if err == nil {
		t.Fatal("expected error for a path-traversal file key")
	}
}

func TestFSBundleStore_ForkIndependentOfSourceEdits(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFSBundleStore(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx := context.Background()

	ref, err := store.Write(ctx, "", map[string][]byte{"manifest.yaml": []byte("name: yandex\n")})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	forked, err := store.Fork(ctx, ref)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}

	if _, err := store.Write(ctx, "", map[string][]byte{"manifest.yaml": []byte("name: mutated\n")}); err != nil {
		t.Fatalf("write: %v", err)
	}

	forkedFiles, err := store.Read(ctx, forked)
	if err != nil {
		t.Fatalf("read forked: %v", err)
	}
	if string(forkedFiles["manifest.yaml"]) != "name: yandex\n" {
		t.Fatalf("fork contents changed after unrelated write: %q", forkedFiles["manifest.yaml"])
	}
}
